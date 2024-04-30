#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

echo "oc is being installed"
if ! oc version; then
  curl -LO https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/latest/openshift-client-linux-amd64-rhel9.tar.gz
  tar -xzf openshift-client-linux-amd64-rhel9.tar.gz
  chmod +x oc kubectl
  mv oc kubectl /usr/local/bin/
fi

echo "krew is being installed"
if ! oc krew version; then
  (
    set -x; cd "$(mktemp -d)" &&
    OS="$(uname | tr '[:upper:]' '[:lower:]')" &&
    ARCH="$(uname -m | sed -e 's/x86_64/amd64/' -e 's/\(arm\)\(64\)\?.*/\1\2/' -e 's/aarch64$/arm64/')" &&
    KREW="krew-${OS}_${ARCH}" &&
    curl -fsSLO "https://github.com/kubernetes-sigs/krew/releases/latest/download/${KREW}.tar.gz" &&
    tar zxvf "${KREW}.tar.gz" &&
    ./"${KREW}" install krew
  )
fi
