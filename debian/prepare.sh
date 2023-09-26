#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
ROOT_DIR="$SCRIPT_DIR/.."

pushd "$ROOT_DIR"

rm -f VERSION
version=$(scripts/get-version | sed 's/+.*$//')
srcver=$(echo "$version" | sed -e 's,\(^[^+]\+\)-,\1~,; s,-,.,g')

echo "$srcver" > VERSION
git add -f VERSION

sed "s/@golang_version@/$(python3 -c "import yaml; f = open('.circleci/config.yml', 'r'); y = yaml.safe_load(f); v = y['parameters']['go-version']['default']; print('.'.join(v.split('.')[:2]))")/" debian/control.in > debian/control
git add -f debian/control

dch -D "unstable" -v "${srcver}" -M "See: https://github.com/sylabs/singularity/blob/main/CHANGELOG.md"
dch -r "" -M
git add debian/changelog

echo '!third_party/**' >> .gitignore
git add .gitignore

submodules="$(git submodule -q foreach 'echo "$displaypath"')"
for submodule in $submodules; do
	mv "$submodule" "$submodule"_tmp
	git submodule deinit "$submodule"
	git rm "$submodule"
	mv "$submodule"_tmp "$submodule"
	rm -rf "$submodule"/.git
	git add "$submodule"
	echo "Deinited $submodule"
done

GOPATH="$ROOT_DIR"/go-deps go mod vendor && git add vendor && chmod -R ug+w go-deps && rm -rf go-deps

popd
