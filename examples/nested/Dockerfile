FROM ubuntu:24.10

RUN ln -s /usr/share/zoneinfo/UTC /etc/localtime && \
    apt-get -yq update && \
    apt-get -yq install curl jq tzdata && \
    PACKAGE_URL=$(curl -s https://api.github.com/repos/sylabs/singularity/releases/latest | jq -r ".assets[] | select(.name | test(\"noble\")) | .browser_download_url") && \
    curl -L -o /tmp/nested-singularity.deb "${PACKAGE_URL}" && \
    apt-get -yq install /tmp/nested-singularity.deb && \
    rm /tmp/nested-singularity.deb && \
    rm -rf /var/cache/apt/archives /var/lib/apt/lists/* && \
    singularity config global --set "allow setuid" "no"

ENTRYPOINT ["/usr/bin/singularity"]
CMD ["version"]
