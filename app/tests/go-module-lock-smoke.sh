#!/usr/bin/env bash
# Release-blocking reviewed-dependency inventory contract. This stays offline
# so it can run with normal source checks. The pinned `go mod tidy -diff`
# gate is authoritative for the lock. The release SBOM is narrower: it derives
# authenticated runtime dependencies from matching build metadata in all five
# released Linux binaries, so graph-only and upstream test modules are never
# promoted into runtime DEPENDS_ON relationships merely because they exist.
set -euo pipefail

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
MOD="$ROOT/go.mod"
SUM="$ROOT/go.sum"
NOTICES="$ROOT/../THIRD_PARTY_NOTICES.md"
LICENSES="$ROOT/../third_party_licenses"
fail(){ printf 'FAIL: %s\n' "$*" >&2; exit 1; }
need_line(){ grep -Fqx -- "$2" "$1" || fail "missing exact dependency lock: $2"; }
need_notice(){ grep -Fq -- "| \`$1\` | \`$2\` | $3 |" "$NOTICES" || fail "THIRD_PARTY_NOTICES.md does not cover $1 $2 as $3"; }
need_file(){ [ -s "$1" ] || fail "required third-party license text is missing or empty: ${1#$ROOT/../}"; }

[ -s "$SUM" ] || fail 'go.sum must be present and non-empty in every source handoff'
need_line "$MOD" 'require github.com/unraid/apprise-go v0.2.6'

# Apprise-Go must remain a tagged module dependency, never a moving branch,
# a local path override, or an unchecked replacement.
if grep -Eq '^[[:space:]]*replace[[:space:]].*github\.com/unraid/apprise-go|^[[:space:]]*github\.com/unraid/apprise-go[[:space:]].*=>' "$MOD"; then
  fail 'Apprise-Go must not be replaced by a local path, branch, or alternate module'
fi
if grep -Eq 'github\.com/unraid/apprise-go[[:space:]]+v?(main|master|head|latest|dev)([[:space:]]|$)' "$MOD"; then
  fail 'Apprise-Go must use the exact reviewed v0.2.6 tag'
fi

need_line "$SUM" 'github.com/unraid/apprise-go v0.2.6 h1:N+nZ6jl/dW/HAVEQNnrmcbiW42DoHBN6v2c73rMUGE0='
need_line "$SUM" 'github.com/unraid/apprise-go v0.2.6/go.mod h1:GC0RrJAgW5/giJaJwP8FV9Dd/XMBc5BbQE2wCmDoj+w='
need_line "$SUM" 'github.com/gomarkdown/markdown v0.0.0-20260417124207-7d523f7318df h1:Mwihr/o+v4L5h56rwHLOE20+hh7Okhwno5BHz3zDuao='

# Generated browser-bundle minification is part of the compiled server's asset
# generator. Keep both the direct minifier and its parser dependency pinned,
# authenticated, noticed, and free of local replacements.
need_line "$MOD" 'require github.com/tdewolff/minify/v2 v2.24.13'
for module in github.com/tdewolff/minify/v2 github.com/tdewolff/parse/v2; do
  escaped_module=${module//./\.}
  if grep -Eq "^[[:space:]]*replace[[:space:]].*${escaped_module}|^[[:space:]]*${escaped_module}[[:space:]].*=>" "$MOD"; then
    fail "${module} must not be replaced by a local or alternate module"
  fi
done
need_line "$SUM" 'github.com/tdewolff/minify/v2 v2.24.13 h1:xrcF7gKDnUszseEY9WX9mUlZII2v2Go/QAcAwRASw58='
need_line "$SUM" 'github.com/tdewolff/minify/v2 v2.24.13/go.mod h1:emvwoYeIl8bfAKqRU5ww95LX9Gpggpqv/naal9a8Yq0='
need_line "$SUM" 'github.com/tdewolff/parse/v2 v2.8.12 h1:5BBjfaCv482v3nltlS0u6wH1xJaxjR6ofDrWttNvROg='
need_line "$SUM" 'github.com/tdewolff/parse/v2 v2.8.12/go.mod h1:Hwlni2tiVNKyzR1o6nUs4FOF07URA+JLBLd6dlIXYqo='
need_line "$SUM" 'github.com/tdewolff/test v1.0.11/go.mod h1:XPuWBzvdUzhCuxWO1ojpXsyzsA5bFoS3tO/Q3kFuTG8='
need_line "$SUM" 'github.com/tdewolff/test v1.0.12 h1:7F21DqIajswxuche0geHdrUZRCWE4oko4b7bcmkkrxk='
need_line "$SUM" 'github.com/tdewolff/test v1.0.12/go.mod h1:XPuWBzvdUzhCuxWO1ojpXsyzsA5bFoS3tO/Q3kFuTG8='
if grep -Fq '| `github.com/tdewolff/test` |' "$NOTICES"; then
  fail 'the upstream minifier test helper is graph-only and must not be promoted into release notices or runtime SBOM dependencies'
fi
need_notice 'github.com/tdewolff/minify/v2' 'v2.24.13' 'MIT'
need_notice 'github.com/tdewolff/parse/v2' 'v2.8.12' 'MIT'

# Only modules required by the app's imported package graph remain explicit.
# Do not retain a stale indirect requirement merely because it also appears in
# the broader module metadata graph; `go mod tidy -diff` blocks that drift.
for spec in \
  'golang.org/x/net v0.56.0 BSD 3-Clause' \
  'golang.org/x/crypto v0.53.0 BSD 3-Clause'; do
  module=${spec%% *}
  rest=${spec#* }
  version=${rest%% *}
  license=${rest#* }
  escaped_module=${module//./\.}
  escaped_version=${version//./\.}
  grep -Eq "^[[:space:]]*${escaped_module}[[:space:]]+${escaped_version}[[:space:]]+//[[:space:]]+indirect[[:space:]]*$" "$MOD" || fail "${module} must be selected at exact reviewed ${version}"
  if grep -Eq "^[[:space:]]*replace[[:space:]].*${escaped_module}|^[[:space:]]*${escaped_module}[[:space:]].*=>" "$MOD"; then
    fail "${module} must not be replaced by a local or alternate module"
  fi
  need_notice "$module" "$version" "$license"
done

# Keep this broader reviewed source/module-graph inventory and its license texts
# in the public handoff. The builder emits only the subset actually linked into
# all five release binaries as SPDX runtime dependencies.
for spec in \
  'github.com/unraid/apprise-go|v0.2.6|BSD 2-Clause' \
  'github.com/gomarkdown/markdown|v0.0.0-20260417124207-7d523f7318df|BSD 2-Clause' \
  'golang.org/x/crypto|v0.53.0|BSD 3-Clause' \
  'golang.org/x/mod|v0.36.0|BSD 3-Clause' \
  'golang.org/x/net|v0.56.0|BSD 3-Clause' \
  'golang.org/x/sync|v0.21.0|BSD 3-Clause' \
  'golang.org/x/sys|v0.46.0|BSD 3-Clause' \
  'golang.org/x/term|v0.44.0|BSD 3-Clause' \
  'golang.org/x/text|v0.38.0|BSD 3-Clause' \
  'golang.org/x/tools|v0.45.0|BSD 3-Clause' \
  'gopkg.in/yaml.v3|v3.0.1|MIT AND Apache-2.0'; do
  IFS='|' read -r module version license <<< "$spec"
  need_notice "$module" "$version" "$license"
done
for license in BSD-2-Clause.txt BSD-3-Clause.txt MIT.txt Apache-2.0.txt OFL-1.1.txt; do
  need_file "$LICENSES/$license"
done

if grep -Fq 'golang.org/x/net v0.54.0' "$MOD" "$SUM"; then
  fail 'vulnerable golang.org/x/net v0.54.0 lock residue must not remain'
fi
if grep -Fq 'golang.org/x/crypto v0.51.0' "$MOD" "$SUM"; then
  fail 'vulnerable golang.org/x/crypto v0.51.0 lock residue must not remain'
fi
need_line "$SUM" 'golang.org/x/crypto v0.53.0 h1:QZ4Muo8THX6CizN2vPPd5fBGHyogrdK9fG4wLPFUsto='
need_line "$SUM" 'golang.org/x/crypto v0.53.0/go.mod h1:DNLU434OwVakk9PzuwV8w62mAJpRJL3vsgcfp4Qnsio='
need_line "$SUM" 'golang.org/x/net v0.56.0 h1:Rw8j/hFzGvJUZwNBXnAtf5sVDVt+65SK2C7IxCxZt5o='
need_line "$SUM" 'golang.org/x/net v0.56.0/go.mod h1:D3Ku6r+V6JROoZK144D2XfMHFcMq/0zSfLelVTCFKec='

printf 'PASS: reviewed Go dependency notices and tidy Go dependency locks are present; the builder will verify the linked release-binary graph\n'
