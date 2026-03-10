#!/usr/bin/env python3
"""Generate OpenAPI spec from main.go route registrations."""
import re
from collections import defaultdict

with open('cmd/replog/main.go') as f:
    content = f.read()

api_section = content[content.find('r.Route("/api"'):]
routes = re.findall(r'r\.(Get|Post|Put|Delete)\("([^"]+)"', api_section)

paths = defaultdict(list)
for method, path in routes:
    if 'Accept' in path:
        continue
    paths[path].append(method.lower())

def get_tag(path):
    if '/passkeys/' in path: return 'Passkeys'
    if '/auth/' in path or path in ['/login', '/logout', '/me']: return 'Auth'
    if '/admin/' in path or '/catalog/' in path: return 'Admin'
    if '/avatars/' in path: return 'Avatars'
    if '/setup/' in path: return 'Auth'
    if '/notifications' in path: return 'Notifications'
    if '/preferences' in path: return 'Preferences'
    p = path
    if '/equipment' in p and '/athletes/' not in p: return 'Equipment'
    if '/users' in p: return 'Users'
    if '/programs' in p and '/athletes/' not in p: return 'Programs'
    if '/exercises' in p and '/athletes/' not in p: return 'Exercises'
    if '/dashboard' in p: return 'Dashboard'
    if '/reviews/' in p: return 'Reviews'
    if '/athletes' in p: return 'Athletes'
    return 'Other'

def get_summary(method, path):
    parts = [p for p in path.split('/') if p and not p.startswith('{')]
    last = parts[-1].replace('-', ' ').title() if parts else ''
    segs = path.split('/')
    has_trailing_param = len(segs) > 0 and segs[-1].startswith('{')
    if method == 'get' and not has_trailing_param:
        return 'List ' + last
    if method == 'post' and not has_trailing_param:
        return last
    if method == 'get':
        return 'Get ' + last
    if method == 'put':
        return 'Update ' + last
    if method == 'delete':
        return 'Delete ' + last
    return last

order = {'get': 0, 'post': 1, 'put': 2, 'delete': 3}

lines = []
lines.append('openapi: "3.0.3"')
lines.append('info:')
lines.append('  title: RepLog API')
lines.append('  description: REST API for the RepLog workout tracking application.')
lines.append('  version: "1.0.0"')
lines.append('servers:')
lines.append('  - url: /api')
lines.append('    description: Local server')
lines.append('tags:')
for tag in ['Auth', 'Dashboard', 'Athletes', 'Exercises', 'Programs',
            'Equipment', 'Users', 'Notifications', 'Preferences',
            'Passkeys', 'Avatars', 'Reviews', 'Admin']:
    lines.append(f'  - name: {tag}')
lines.append('paths:')

for path in sorted(paths.keys()):
    lines.append(f'  {path}:')
    methods = sorted(paths[path], key=lambda m: order.get(m, 5))
    for method in methods:
        tag = get_tag(path)
        summary = get_summary(method, path)
        op_id = method + path.replace('/', '_').replace('{', '').replace('}', '').replace('-', '_')
        lines.append(f'    {method}:')
        lines.append(f'      tags: [{tag}]')
        lines.append(f'      summary: "{summary}"')
        lines.append(f'      operationId: {op_id}')
        params = re.findall(r'\{(\w+)\}', path)
        if params:
            lines.append('      parameters:')
            for p in params:
                ptype = 'string' if p == 'token' else 'integer'
                lines.append(f'        - name: {p}')
                lines.append(f'          in: path')
                lines.append(f'          required: true')
                lines.append(f'          schema:')
                lines.append(f'            type: {ptype}')
        lines.append('      responses:')
        lines.append("        '200':")
        lines.append('          description: Success')

with open('docs/openapi.yaml', 'w') as f:
    f.write('\n'.join(lines) + '\n')

print(f'Generated {len(lines)} lines, {len(routes)} operations across {len(paths)} paths')
