import base64
import json
import os
import re
import shutil
import subprocess
import sys

import requests
import semver
import yaml

from github import Github
from nested_lookup import nested_lookup

def _get_all_from_image(image):
    # Examples of image:
    # flannel/flannel
    # quay.io/skopeo/stable
    # k8s.gcr.io/pause
    # ghcr.io/epinio/epinio-server
    # gcr.io/cloud-provider-vsphere/csi/release/syncer
    # registry.suse.com/bci/bci-busybox
    # dockerhub
    splitted_image = image.split("/")
    if len(splitted_image) == 2 and not '.' in splitted_image[0]:
        return "dockerhub", splitted_image[0], splitted_image[1]
    # registry.k8s.io/pause
    if len(splitted_image) == 2 and '.' in splitted_image[0]:
        return splitted_image[0], "", splitted_image[1]
    # gcr.io/cloud-provider-vsphere/cpi/release/manager
    if len(splitted_image) > 3 and '.' in splitted_image[0]:
        return splitted_image[0], splitted_image[1], "/".join(splitted_image[2:])
    return splitted_image[0], splitted_image[1], splitted_image[2]

def _get_image_tags(registry, namespace, package, nexturl, page=1):
    #print(f"registry({registry}),namespace({namespace}),package({package})")
    s = requests.Session()
    s.mount("https://", requests.adapters.HTTPAdapter(max_retries=requests.adapters.Retry(total=10, backoff_factor=1, status_forcelist=[ 500, 502, 503, 504 ])))

    # We set the variables/parameters based on the registry we need to query
    registry_url = ""
    params = {}
    headers = {}
    match registry:
        case 'dockerhub':
            registry_url = f"https://registry.hub.docker.com/v2/namespaces/{namespace}/repositories/{package}/tags"
            params={"page": page, "page_size": 100}
        case 'quay.io':
            registry_url = f"https://quay.io/api/v1/repository/{namespace}/{package}/tag/"
            params={"page": page, "page_size": 100}
        case 'registry.k8s.io':
            if namespace != '':
                registry_url = f"https://registry.k8s.io/v2/{namespace}/{package}/tags/list"
            else:
                # Example: registry.k8s.io/pause
                registry_url = f"https://registry.k8s.io/v2/{package}/tags/list"
        case 'registry.suse.com':
            # We need to request a token first
            base_registry_url = "https://registry.suse.com"
            scope = f"scope=repository:{namespace}/{package}:pull"
            token_url = f"{base_registry_url}/auth?service=SUSE+Linux+Docker+Registry"
            token_res = requests.get(url=token_url, params=scope)
            token_data = token_res.json()
            access_token = token_data['access_token']
            headers['Authorization'] = 'Bearer ' + access_token
            registry_url = f"{base_registry_url}/v2/{namespace}/{package}/tags/list"
        case 'ghcr.io':
            ghcr_token = base64.b64encode(github_token.encode("utf-8")).decode("utf-8")
            headers['Authorization'] = 'Bearer ' + ghcr_token
            registry_url = f"https://ghcr.io/v2/{namespace}/{package}/tags/list"
            if nexturl != "":
                registry_url = f"https://ghcr.io{nexturl}"
        case 'gcr.io':
            registry_url = f"https://gcr.io/v2/{namespace}/{package}/tags/list"
        case _:
            print(f"Unrecognized registry: {registry}")
            sys.exit(1)

    result = s.get(registry_url,
        params=params,
        headers=headers,
        timeout=10.0,
    )
    result.raise_for_status()
    data = result.json()
    # Return the correct data based on the registry we queried (or paginate if registry/query requires it)
    if registry == 'dockerhub':
        tags = [tag['name'] for tag in data['results']]
        #print(f"tags for {namespace}/{package}: {tags}")
        if not data['next']:
            return tags
        return tags + _get_image_tags(registry, namespace, package, "", page=page + 1)
    if registry == 'quay.io':
        tags = [tag['name'] for tag in data['tags']]
        if not data['has_additional']:
            return tags
        return tags + _get_image_tags(registry, namespace, package, "", page=page + 1)
    if registry in ['registry.k8s.io', 'registry.suse.com', 'gcr.io']:
        return data['tags']
    if registry == 'ghcr.io':
        tags = data['tags']
        if not 'next' in result.links:
            return tags
        return tags + _get_image_tags(registry, namespace, package, result.links['next']['url'], page=page)
    print(f"Unrecognized registry: {registry}")
    sys.exit(1)

# Function to search textfile content
def _image_tag_already_exist(filetext, image, tag):
    matches = re.findall(fr"{image}\s.*?\s{tag}\n", filetext)
    if matches:
        return True
    return False

# Function to retrieve images from helm chart using helm template
def _extract_unique_images_from_helm_template(repo_name, chart, chart_values='', kube_version='', devel=False, version=''):
    rendered_chart = ''
    if kube_version != '':
        kube_version = f"--kube-version {kube_version}"
    develArg = ''
    if devel:
        develArg = '--devel'
    versionArg = ''
    if version != '':
        versionArg = f"--version {version}"
    if chart.startswith('oci://'):
        rendered_chart = subprocess.check_output(
            f"helm template {kube_version} {chart_values} {chart} {develArg} {versionArg}",
            stderr=subprocess.DEVNULL,
            shell=True,
        ).decode()
    else:
        chart = f"{repo_name}/{chart}"
        rendered_chart = subprocess.check_output(
            f"helm template {kube_version} {chart_values} {repo_name} {chart} {develArg} {versionArg}",
            stderr=subprocess.DEVNULL,
            shell=True,
        ).decode()
    yaml_output = yaml.safe_load_all(rendered_chart)
    helm_template_images = []
    for doc in yaml_output:
        yaml_images = nested_lookup(key='image',document=doc,wild=True)
        for image in yaml_images:
            if not isinstance(image, str):
                continue
            helm_template_images.append(image) if image not in helm_template_images else helm_template_images
    helm_template_images = [i.split('@')[0] for i in helm_template_images]
    return helm_template_images

def _extract_images_from_dict(d, images=[]):
    if isinstance(d, dict):
        for k, v in d.items():
            #if not isinstance(v, dict):
            if all(k in d for k in ('repository','tag')):
                image = ''
                if 'registry' in d:
                    image = d['registry']
                image += d['repository']
                image += f":{d['tag']}"
                images.append(image) if not image in images else images
            else:
                _extract_images_from_dict(v, images)
    elif hasattr(d, '__iter__') and not isinstance(d, str):
        for item in d:
            _extract_images_from_dict(item, images)
    elif isinstance(d, str):
        image_regex = r"^(?:(?=[^:\/]{4,253})(?!-)[a-zA-Z0-9-]{1,63}(?<!-)(?:\.(?!-)[a-zA-Z0-9-]{1,63}(?<!-))*(?::[0-9]{1,5})?/)?((?![._-])(?:[a-z0-9._-]*)(?<![._-])(?:/(?![._-])[a-z0-9._-]*(?<![._-]))*)(?::(?![.-])[a-zA-Z0-9_.-]{1,128})?(@.*)?$"
        if re.match(image_regex, d):
            images.append(d) if not d in images else images
    # for debugging
    #else:
    #    print(f"unknown type: {d}")
    return images

def _extract_unique_images_from_helm_values(repo_name, chart, devel=False, version=''):
    if not chart.startswith('oci://'):
        chart = f"{repo_name}/{chart}"
    develArg = ''
    if devel:
        develArg = '--devel'
    versionArg = ''
    if version != '':
        versionArg = f"--version {version}"
    chart_values = subprocess.check_output(
        f"helm show values {chart} {develArg} {versionArg}",
        stderr=subprocess.DEVNULL,
        shell=True,
    ).decode()
    chart_values_yaml = yaml.safe_load_all(chart_values)
    found_images = []
    for chart_values_doc in chart_values_yaml:
        chart_yaml_images = nested_lookup(key='image',document=chart_values_doc,wild=True,with_keys=True)
        extracted_images = _extract_images_from_dict(chart_yaml_images['image'], [])
        found_images.append(extracted_images)
    extracted_images = [i.split('@')[0] for i in extracted_images]
    return extracted_images

# using an access token
github_token = os.getenv('GITHUB_TOKEN')
g = Github(github_token)

# Configuration file with repositories
f = open('config.json', 'r')
data = json.load(f)
# Closing file
f.close()

# Read in images-list to check if image/tag already exist
textfile = open('../images-list', 'r')
filetext = textfile.read()
textfile.close()

alldict = {}

# loop through each repository
for (key, values) in data.items():
    alldict[key] = {}
    # determine version source for the image tags
    repoString = values['versionSource'].split(":", 1)
    if values['versionSource'] != 'registry' and values['versionSource'] != 'helm-oci':
        if len(repoString) != 2:
            print(f"Can not extract proper version source from versionSource {values['versionSource']} for image {key}")
            sys.exit(1)

    found_releases = []
    image_denylist = []
    additional_version_filter = []
    extracted_images = []

    match repoString[0]:
        case 'helm-latest' | 'helm-oci':
            # check if helm binary is available
            helm_path = shutil.which('helm')

            if helm_path is None:
                print("no executable found for command 'helm'")
                sys.exit(1)

            if repoString[0] == 'helm-latest':
                helm_repo = repoString[1]
            repo_name = key
            if helm_repo.startswith('https://'):
                subprocess.check_call(f"helm repo add {repo_name} {helm_repo}",
                    shell=True,
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.STDOUT)
                subprocess.check_call('helm repo update',
                    shell=True,
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.STDOUT)
            if 'imageDenylist' in values:
                image_denylist = values['imageDenylist']
            if 'additionalVersionFilter' in values:
                additional_version_filter = values['additionalVersionFilter']
            for chart in values['helmCharts']:
                devel = False
                if 'devel' in values['helmCharts'][chart]:
                    devel = values['helmCharts'][chart]['devel']
                version_filter = ''
                if 'versionFilter' in values['helmCharts'][chart]:
                    version_filter = values['helmCharts'][chart]['versionFilter']
                if 'chartConfig' in values['helmCharts'][chart]:
                    for arg_key in values['helmCharts'][chart]['chartConfig']:
                        chart_values = ''
                        if 'values' in values['helmCharts'][chart]['chartConfig'][arg_key]:
                            chart_values_config = values['helmCharts'][chart]['chartConfig'][arg_key]['values']
                            chart_values = ["--set " + value for value in chart_values_config]
                            chart_values = ' '.join(chart_values)
                        kube_version = ''
                        if 'kubeVersion' in values['helmCharts'][chart]['chartConfig'][arg_key]:
                            kube_version = values['helmCharts'][chart]['chartConfig'][arg_key]['kubeVersion']
                        extracted_images = _extract_unique_images_from_helm_template(repo_name, chart, chart_values, kube_version, devel, version_filter)
                        found_releases.extend(extracted_images)
                        if additional_version_filter:
                            for additional_version in additional_version_filter:
                                extracted_images = _extract_unique_images_from_helm_template(repo_name, chart, chart_values, kube_version, devel, additional_version)
                                found_releases.extend(extracted_images)
                else:
                    extracted_images = _extract_unique_images_from_helm_template(repo_name, chart, devel=devel, version=version_filter)
                    found_releases.extend(extracted_images)
                    if additional_version_filter:
                        for additional_version in additional_version_filter:
                            extracted_images = _extract_unique_images_from_helm_template(repo_name, chart, chart_values, kube_version, devel, additional_version)
                            found_releases.extend(extracted_images)

            extracted_images = _extract_unique_images_from_helm_values(repo_name, chart, devel=devel, version=version_filter)
            found_releases.extend(extracted_images)
            if additional_version_filter:
                for additional_version in additional_version_filter:
                    extracted_images = _extract_unique_images_from_helm_values(repo_name, chart, devel=devel, version=additional_version)
                    found_releases.extend(extracted_images)
        case 'github-releases':
            repo = g.get_repo(repoString[1])
            # Get all the releases used as source for the tag
            releases = repo.get_releases()
            if 'versionConstraint' in values:
                constraint = values['versionConstraint']
            for release in releases:
                if not release.prerelease and not release.draft and semver.VersionInfo.isvalid(release.tag_name.removeprefix('v')):
                    if 'versionFilter' in values:
                        if not re.search(values['versionFilter'], release.tag_name):
                            continue
                    if 'versionConstraint' in values:
                        # Checking constraint
                        if not semver.match(release.tag_name.removeprefix('v'), constraint):
                            continue
                    found_releases.append(release.tag_name)
        case 'github-latest-release':
            repo = g.get_repo(repoString[1])
            # Get all the releases used as source for the tag
            latest_release = repo.get_latest_release()
            found_releases.append(latest_release.tag_name)
        case 'registry':
            registry, namespace, package  = _get_all_from_image(values['images'][0])
            image_tags = _get_image_tags(registry, namespace, package, '')
            found_images = []
            if 'versionFilter' in values:
                for image_tag in image_tags:
                    if not re.search(values['versionFilter'], image_tag):
                        continue
                    found_images.append(image_tag)
            else:
                found_images.extend(image_tags)
            if 'latest_entry' in values:
                found_releases = [found_images[0]]
            elif 'latest' in values:
                found_images.sort(key = lambda x: [int(y.removeprefix('v')) for y in x.split('.')])
                found_releases = [found_images[-1]]
            else:
                found_releases.extend(found_images)
        case _:
            print(f"Version source {repoString[0]} is not supported")
            continue

    # deduplicate
    found_releases = set(found_releases)

    match repoString[0]:
        case 'helm-latest' | 'helm-oci':
            alldict[key]['full_images'] = []
            for full_image in found_releases:
                full_image = full_image.removeprefix('docker.io/')
                if full_image.startswith('rancher/'):
                    continue
                splitted_image = full_image.split(':')
                if len(splitted_image) == 2:
                    if splitted_image[1] == 'latest':
                        continue
                    if splitted_image[0] in image_denylist:
                        continue
                    if not _image_tag_already_exist(filetext, splitted_image[0], splitted_image[1]):
                        alldict[key]['full_images'].append(full_image)
            alldict[key]['full_images'].sort()
        case _:
            alldict[key]['images'] = []
            alldict[key]['tags'] = []

            for image in values['images']:
                if image.startswith('rancher/'):
                    continue
                for tag in found_releases:
                    if not _image_tag_already_exist(filetext, image, tag):
                        if image not in alldict[key]['images']:
                            alldict[key]['images'].append(image)
                        if tag not in alldict[key]['tags']:
                            alldict[key]['tags'].append(tag)
                # sort because we match existing pull requests on images/tags in pull request title
                alldict[key]['images'].sort()
                alldict[key]['tags'].sort()

# Print json
dictjson = json.dumps(alldict)
print(dictjson)
