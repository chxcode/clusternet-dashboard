"""Synchronize the dashboard release pins in the source chart and install guides."""
import argparse
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CHART = 'charts/clusternet-dashboard'
IMAGE_REPOSITORY = 'changxuan/clusternet-dashboard'
# Each expression identifies exactly one release pin. Upstream Clusternet
# references and unrelated examples in the guides have independent versions.
PINS = {
    f'{CHART}/Chart.yaml': [
        ('version', r'^version: (?P<pin>\S+)$', False),
        ('appVersion', r'^appVersion: "(?P<pin>[^"]+)"$', True),
    ],
    f'{CHART}/values.yaml': [
        ('image.tag', r'^image:\n(?:[ ]+[^\n]*\n)*?  tag: (?P<pin>\S+)$', True),
    ],
}
for guide in ('README.md', 'README.zh-CN.md'):
    PINS[guide] = [
        ('OCI install version', r'^  oci://registry-1\.docker\.io/changxuan/clusternet-dashboard \\\n  --version (?P<pin>\S+) \\$', False),
        ('dashboard image', r'`docker\.io/changxuan/clusternet-dashboard:(?P<pin>[^`\s]+)`', True),
    ]

def validate_tag(tag):
    number = r'(?:0|[1-9][0-9]*)'
    prerelease = r'[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*'
    match = re.fullmatch(rf'v{number}\.{number}\.{number}(?:-({prerelease}))?', tag)
    if not match or len(tag) > 128 or any(
        part.isdigit() and len(part) > 1 and part.startswith('0')
        for part in (match.group(1) or '').split('.')
    ):
        raise SystemExit(f'Invalid dashboard release tag {tag!r}. Expected e.g. v1.2.3 or v1.2.3-rc.1.')

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    commands.add_parser('set', help='update all tracked pins').add_argument('tag')
    commands.add_parser('check', help='compare all pins to TAG or the source chart version').add_argument('tag', nargs='?')
    args = parser.parse_args()
    files = {}
    fields = []
    for filename, entries in PINS.items():
        path = ROOT / filename
        files[path] = path.read_text(encoding='utf-8')
        for label, pattern, prefixed in entries:
            matches = list(re.finditer(pattern, files[path], re.MULTILINE))
            if len(matches) != 1:
                raise SystemExit(f'{filename}: expected exactly one {label} pin, found {len(matches)}. Check its location and format.')
            fields.append((path, label, matches[0], prefixed))
    # CI publishes this repository. Check the packaged image reference as well as its tag.
    image_repo = re.search(r'^image:\n(?:[ ]+[^\n]*\n)*?  repository: (\S+)$',
                           files[ROOT / CHART / 'values.yaml'], re.MULTILINE)
    if not image_repo or image_repo.group(1) != IMAGE_REPOSITORY:
        raise SystemExit(f'{CHART}/values.yaml: image.repository must match {IMAGE_REPOSITORY!r}.')
    # The source chart version is the first declared pin.
    tag = 'v' + fields[0][2].group('pin') if args.tag is None else args.tag
    validate_tag(tag)

    errors = []
    # Replace the latest match first to keep each earlier offset valid.
    for path, label, match, prefixed in sorted(fields, key=lambda field: field[2].start('pin'), reverse=True):
        expected = tag if prefixed else tag[1:]
        actual = match.group('pin')
        if actual == expected:
            continue
        if args.command == 'check':
            errors.append(f'{path.relative_to(ROOT)}: {label} is {actual!r}, expected {expected!r}')
        else:
            start, end = match.span('pin')
            files[path] = files[path][:start] + expected + files[path][end:]
    if errors:
        raise SystemExit('\n'.join(errors) + f'\nRun: python3 scripts/release-version.py set {tag}')
    if args.command == 'set':
        for path, content in files.items():
            if path.read_text(encoding='utf-8') != content:
                path.write_text(content, encoding='utf-8')
    print(f'Dashboard release pins match {tag}.')

if __name__ == '__main__':
    main()
