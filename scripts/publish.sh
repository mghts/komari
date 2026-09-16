#!/usr/bin/env bash
set -euo pipefail
: "${VERSION:?}" "${IMAGE:?}" "${GITHUB_SHA:?}" "${GH_REPO:?}"
for arch in amd64 arm64; do
  docker load -i "dist/image-$arch.tar"
  docker tag "local/komari:$VERSION-$arch" "$IMAGE:$VERSION-$arch"
  docker push "$IMAGE:$VERSION-$arch"
done
docker buildx imagetools create -t "$IMAGE:$VERSION" "$IMAGE:$VERSION-amd64" "$IMAGE:$VERSION-arm64"
docker buildx imagetools inspect "$IMAGE:$VERSION" --format '{{.Manifest.Digest}}' > dist/image-digest.txt
python3 - <<'PY'
import hashlib, json, os
from pathlib import Path
root = Path('dist')
assets = [root/'komari-linux-amd64', root/'komari-linux-arm64']
(root/'SHA256SUMS').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.name+'\n' for p in assets))
manifest = json.loads(Path('build/baseline.json').read_text())
manifest.update(version=os.environ['VERSION'], commit=os.environ['GITHUB_SHA'],
                image=os.environ['IMAGE']+':'+os.environ['VERSION'],
                digest=(root/'image-digest.txt').read_text().strip(),
                platforms=['linux/amd64','linux/arm64'])
manifest['frontend'] = json.loads(Path('build/frontend.json').read_text())
manifest['agent'] = json.loads(Path('build/agent.json').read_text())
(root/'build-manifest.json').write_text(json.dumps(manifest, indent=2)+'\n')
(root/'RELEASE.md').write_text(
    '## Independent fork build\n\n'
    + 'Source baseline: '+manifest['upstream_version']+'; see build-manifest.json for exact commits.\n\n'
    + 'Image: `'+manifest['image']+'`\n\n'
    + 'Platforms: linux/amd64, linux/arm64.\n\n'
    + 'Go tests and container smoke checks passed on both architectures before publication.\n\n'
    + 'Release assets include SHA256SUMS. RC releases are for validation, not automatic production upgrades.\n')
PY
gh release create "$VERSION" --target "$GITHUB_SHA" --draft --title "$VERSION" --notes-file dist/RELEASE.md
gh release upload "$VERSION" dist/komari-linux-amd64 dist/komari-linux-arm64 dist/SHA256SUMS dist/build-manifest.json
if [[ "$VERSION" == *-* ]]; then
  gh release edit "$VERSION" --draft=false --prerelease --latest=false
else
  gh release edit "$VERSION" --draft=false --prerelease=false --latest
  docker buildx imagetools create -t "$IMAGE:latest" "$IMAGE:$VERSION"
fi
