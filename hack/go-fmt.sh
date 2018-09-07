#!/bin/sh
if [ "$IS_CONTAINER" != "" ]; then
  for TARGET in "${@}"; do
    find "${TARGET}" -name '*.go' ! -path '*/vendor/*' ! -path '*/.build/*' -exec gofmt -s -w {} \+
  done
  git diff --exit-code
else
  docker run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/go/src/sigs.k8s.io/cluster-api-provider-aws:z" \
    --workdir /go/src/sigs.k8s.io/cluster-api-provider-aws \
    --entrypoint sh \
    quay.io/coreos/golang-testing \
    ./hack/go-fmt.sh "${@}"
fi
