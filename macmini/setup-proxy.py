#!/usr/bin/env python3
import json, os

path = os.path.expanduser('~/.docker/config.json')
cfg = {}
if os.path.exists(path):
    with open(path) as f:
        cfg = json.load(f)

cfg['proxies'] = {
    'default': {
        'httpProxy': 'http://host.docker.internal:7890',
        'httpsProxy': 'http://host.docker.internal:7890',
        'noProxy': 'localhost,127.0.0.1'
    }
}

os.makedirs(os.path.dirname(path), exist_ok=True)
with open(path, 'w') as f:
    json.dump(cfg, f, indent=2)

print('done:', path)
