module github.com/sylabs/singularity/v4

go 1.24.3

require (
	github.com/Netflix/go-expect v0.0.0-20220104043353-73e0943537d2
	github.com/ProtonMail/go-crypto v1.3.0
	github.com/adigunhammedolalekan/registry-auth v0.0.0-20200730122110-8cde180a3a60
	github.com/apex/log v1.9.0
	github.com/astromechza/etcpwdparse v0.0.0-20170319193008-f0e5f0779716
	github.com/blang/semver/v4 v4.0.0
	github.com/buger/goterm v1.0.4
	github.com/buger/jsonparser v1.1.1
	github.com/ccoveille/go-safecast v1.6.1
	github.com/containerd/containerd/v2 v2.1.4
	github.com/containerd/continuity v0.4.5
	github.com/containerd/go-runc v1.1.0
	github.com/containerd/platforms v1.0.0-rc.1
	github.com/containernetworking/cni v1.3.0
	github.com/containernetworking/plugins v1.7.1
	github.com/containers/image/v5 v5.36.0
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/cyphar/filepath-securejoin v0.4.1
	github.com/docker/cli v28.3.3+incompatible
	github.com/docker/distribution v2.8.3+incompatible
	github.com/docker/docker v28.3.3+incompatible
	github.com/docker/go-units v0.5.0
	github.com/fatih/color v1.18.0
	github.com/go-log/log v0.2.0
	github.com/gofrs/flock v0.12.1
	github.com/google/go-containerregistry v0.20.6
	github.com/google/uuid v1.6.0
	github.com/gosimple/slug v1.15.0
	github.com/moby/buildkit v0.23.2
	github.com/moby/go-archive v0.1.0
	github.com/moby/sys/sequential v0.6.0
	github.com/moby/sys/user v0.4.0
	github.com/moby/sys/userns v0.1.0
	github.com/moby/term v0.5.2
	github.com/opencontainers/cgroups v0.0.4
	github.com/opencontainers/image-spec v1.1.1
	github.com/opencontainers/runc v1.3.0
	github.com/opencontainers/runtime-spec v1.2.1
	github.com/opencontainers/runtime-tools v0.9.1-0.20230914150019-408c51e934dc
	github.com/opencontainers/selinux v1.12.0
	github.com/opencontainers/umoci v0.5.0
	github.com/pelletier/go-toml/v2 v2.2.4
	github.com/pkg/errors v0.9.1
	github.com/samber/lo v1.51.0
	github.com/sebdah/goldie/v2 v2.7.1
	github.com/seccomp/libseccomp-golang v0.11.0
	github.com/shopspring/decimal v1.4.0
	github.com/sigstore/cosign/v2 v2.5.3
	github.com/sigstore/sigstore v1.9.5
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.9.1
	github.com/spf13/pflag v1.0.7
	github.com/sylabs/json-resp v0.9.5
	github.com/sylabs/oci-tools v0.17.0
	github.com/sylabs/scs-build-client v0.9.20
	github.com/sylabs/scs-key-client v0.7.9
	github.com/sylabs/scs-library-client v1.4.14
	github.com/sylabs/sif/v2 v2.21.1
	github.com/sylabs/squashfs v1.0.4
	github.com/vbauerster/mpb/v8 v8.10.2
	go.etcd.io/bbolt v1.4.2
	golang.org/x/crypto v0.40.0
	golang.org/x/sync v0.16.0
	golang.org/x/sys v0.34.0
	golang.org/x/term v0.33.0
	golang.org/x/text v0.27.0
	google.golang.org/grpc v1.74.2
	gopkg.in/yaml.v3 v3.0.1
	gotest.tools v2.2.0+incompatible
	gotest.tools/v3 v3.5.2
	mvdan.cc/sh/v3 v3.12.0
	tags.cncf.io/container-device-interface v1.0.1
	tags.cncf.io/container-device-interface/specs-go v1.0.0
)

require (
	cel.dev/expr v0.24.0 // indirect
	cloud.google.com/go v0.121.4 // indirect
	cloud.google.com/go/auth v0.16.3 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	cloud.google.com/go/iam v1.5.2 // indirect
	cloud.google.com/go/longrunning v0.6.7 // indirect
	cloud.google.com/go/monitoring v1.24.2 // indirect
	cloud.google.com/go/spanner v1.83.0 // indirect
	cloud.google.com/go/storage v1.56.0 // indirect
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20240806141605-e8a1dd7889d6 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/GoogleCloudPlatform/grpc-gcp-go/grpcgcp v1.5.3 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.29.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.53.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.53.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Microsoft/hcsshim v0.13.0 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/alexflint/go-filemutex v1.3.0 // indirect
	github.com/anchore/go-struct-converter v0.0.0-20250211213226-cce56d595160 // indirect
	github.com/armon/circbuf v0.0.0-20190214190532-5111143e8da2 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/avast/retry-go/v4 v4.6.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cilium/ebpf v0.17.3 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/cncf/xds/go v0.0.0-20250501225837-2ac532fd4443 // indirect
	github.com/containerd/accelerated-container-image v1.3.0 // indirect
	github.com/containerd/cgroups/v3 v3.0.5 // indirect
	github.com/containerd/console v1.0.5 // indirect
	github.com/containerd/containerd/api v1.9.0 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containerd/go-cni v1.1.12 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/nydus-snapshotter v0.15.2 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.17.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/containerd/typeurl/v2 v2.2.3 // indirect
	github.com/containers/storage v1.59.0 // indirect
	github.com/coreos/go-iptables v0.8.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/creack/pty v1.1.24 // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20241213102144-19d51d7fe467 // indirect
	github.com/digitorus/pkcs7 v0.0.0-20250730155240-ffadbf3f398c // indirect
	github.com/digitorus/timestamp v0.0.0-20250524132541-c45532741eea // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker-credential-helpers v0.9.3 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.32.4 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/garyburd/redigo v1.6.4 // indirect
	github.com/globocom/go-buffer v1.2.2 // indirect
	github.com/go-chi/chi v4.1.2+incompatible // indirect
	github.com/go-jose/go-jose/v4 v4.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/analysis v0.23.0 // indirect
	github.com/go-openapi/errors v0.22.1 // indirect
	github.com/go-openapi/jsonpointer v0.21.1 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/loads v0.22.0 // indirect
	github.com/go-openapi/runtime v0.28.0 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/strfmt v0.23.0 // indirect
	github.com/go-openapi/swag v0.23.1 // indirect
	github.com/go-openapi/validate v0.24.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/certificate-transparency-go v1.3.2 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	github.com/gorilla/handlers v1.5.2 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-immutable-radix/v2 v2.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/in-toto/attestation v1.1.2 // indirect
	github.com/in-toto/in-toto-golang v0.9.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20250417080101-5f8cf70e8c5f // indirect
	github.com/jedisct1/go-minisign v0.0.0-20241212093149-d2f9f49435c7 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/letsencrypt/boulder v0.20250728.0 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mattn/go-shellwords v1.0.12 // indirect
	github.com/mdlayher/packet v1.1.2 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/patternmatcher v0.6.0 // indirect
	github.com/moby/sys/atomicwriter v0.1.0 // indirect
	github.com/moby/sys/mount v0.3.4 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/reexec v0.1.0 // indirect
	github.com/moby/sys/signal v0.7.1 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/networkplumbing/go-nft v0.4.0 // indirect
	github.com/nozzle/throttler v0.0.0-20180817012639-2ea982251481 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/package-url/packageurl-go v0.1.3 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/petermattis/goid v0.0.0-20250721140440-ea1c0173183e // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.23.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.65.0 // indirect
	github.com/prometheus/procfs v0.17.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rootless-containers/proto/go-proto v0.0.0-20230421021042-4cd87ebadd67 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/safchain/ethtool v0.6.1 // indirect
	github.com/sagikazarmark/locafero v0.10.0 // indirect
	github.com/sasha-s/go-deadlock v0.3.5 // indirect
	github.com/sassoftware/relic v7.2.1+incompatible // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.9.0 // indirect
	github.com/sergi/go-diff v1.4.0 // indirect
	github.com/shibumi/go-pathspec v1.3.0 // indirect
	github.com/sigstore/protobuf-specs v0.5.0 // indirect
	github.com/sigstore/rekor v1.3.10 // indirect
	github.com/sigstore/rekor-tiles v0.1.7-0.20250624231741-98cd4a77300f // indirect
	github.com/sigstore/sigstore-go v1.1.0 // indirect
	github.com/sigstore/timestamp-authority v1.2.8 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spdx/tools-golang v0.5.5 // indirect
	github.com/spf13/afero v1.14.0 // indirect
	github.com/spf13/cast v1.9.2 // indirect
	github.com/spf13/viper v1.20.1 // indirect
	github.com/spiffe/go-spiffe/v2 v2.5.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d // indirect
	github.com/therootcompany/xz v1.0.1 // indirect
	github.com/theupdateframework/go-tuf v0.7.0 // indirect
	github.com/theupdateframework/go-tuf/v2 v2.1.1 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/tonistiigi/dchapes-mode v0.0.0-20250318174251-73d941a28323 // indirect
	github.com/tonistiigi/fsutil v0.0.0-20250605211040-586307ad452f // indirect
	github.com/tonistiigi/go-archvariant v1.0.0 // indirect
	github.com/tonistiigi/go-csvvalue v0.0.0-20240814133006-030d3b2625d0 // indirect
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea // indirect
	github.com/tonistiigi/vt100 v0.0.0-20240514184818-90bafcd6abab // indirect
	github.com/transparency-dev/formats v0.0.0-20250728090553-b6e25248e362 // indirect
	github.com/transparency-dev/merkle v0.0.2 // indirect
	github.com/transparency-dev/tessera v0.2.1-0.20250610150926-8ee4e93b2823 // indirect
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701 // indirect
	github.com/ulikunitz/xz v0.5.12 // indirect
	github.com/vbatts/go-mtree v0.5.4 // indirect
	github.com/vbatts/tar-split v0.12.1 // indirect
	github.com/vishvananda/netlink v1.3.1 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/zeebo/errs v1.4.0 // indirect
	go.mongodb.org/mongo-driver v1.17.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.37.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.62.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.62.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.62.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.37.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.37.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.37.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.37.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.37.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.opentelemetry.io/proto/otlp v1.7.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/exp v0.0.0-20250718183923-645b1fa84792 // indirect
	golang.org/x/mod v0.26.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	google.golang.org/api v0.244.0 // indirect
	google.golang.org/genproto v0.0.0-20250728155136-f173205681a0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250728155136-f173205681a0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250728155136-f173205681a0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	sigs.k8s.io/knftables v0.0.18 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)
