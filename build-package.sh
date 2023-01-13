#!/bin/bash 

ARCH=$(go env GOARCH)
BUILD="build"
CONFIG="packaging/nfpm.yaml"
DIST="dist"
PKG=""
NAME="xk6-disruptor"
OS=$(go env GOOS)
VERSION="latest"

function usage() {
cat << EOF

builds binary and creates an installation package

$0 [OPTIONS] COMMAND

Commands:
  build Builds the binary for the given architecture and operating system
  pack  Packages the binary for the given architecture and operating system in the given format.
        Builds the binary if it does not exist
  all   Builds and packs all supported combinations of architecture, operating system and package format

options:
  -a, --arch: target architecture (valid option amd64, arm64. Defaults to GOARCH)
  -b, --build: directory for building binaries (defaults to 'build. Created if it does not exist)
  -c, --config: nfpm config file (defaults to packaging/nfpm.yaml) 
  -d, --dist: directory to place packages (defaults to 'dist'. Created if it does not exist)
  -n, --name: package base name. Defaults to 'xk6-disruptor'
  -o, --os: target operating systems (valid options linux, darwing, windows. Defaults to GOOS)
  -p, --pkg: package format (valid options: deb, rpm, tgz)
  -v, --version: package version in semver formatf
  -y, --binary: name of the binary (default is name-os-arch)


EOF
}

# Prints an error message the usage help and exits with error
function error () {
   echo $1
   usage
   exit 1
}

# Builds a binary for a target architecture and operating system
# Arguments:
# $1 os
# $2 arch
# $3 version
# $4 binary
function build() {
  local os=$1
  local arch=$2
  local version=$3
  local binary=$4
  if [[ -z $binary ]]; then
    binary="$NAME-$os-$arch"
  fi

  #start sub shell to create its own environment
  (
   if [[ $os == "linux" ]]; then # disable cross-compiling for linux
     export CGO_ENABLED=0
   fi

   export GOARCH=$arch
   export GOOS=$os
   export XK6_BUILD_FLAGS='-ldflags "-X github.com/grafana/xk6-disruptor/pkg/internal/consts.Version='${version}'"'
   xk6 build --with $(go list -m)=. --with github.com/grafana/xk6-kubernetes --output $BUILD/$binary
  )
}


# Creates a package for a target architecture and operating system.
# Requires the package version
# Builds the binary if does not exists
#
# Arguments:
# $1 os
# $2 arch
# $3 pkg
# $4 version
# $5 binary
function package() {
  local os=$1
  local arch=$2
  local pkg=$3
  local version=$4
  local binary=$5

  if [[ -z $binary ]]; then
    binary="$NAME-$os-$arch"
  fi

  if [[ ! -e $BUILD/$binary ]]; then 
    build $os $arch $version $binary
  fi
  
  case $pkg in
    deb|rpm)
      if [[ ! $os == "linux" ]]; then
        error "unsoported operating system for package format $pkg: $os"
      fi

      if [[ ! $arch == "amd64" ]]; then
        error "unsoported architecture for package format $pkg: $os"
      fi

      (
       # nfpm does not support variable subsitution in paths so we must run in the build directory
       pushd $BUILD

       export PKG_VERSION=$version
       target="$DIST/$NAME-$version-$os-$arch.$pkg"
       nfpm package --config $CONFIG --packager $pkg --target $target

       popd
      )
      ;;
    tgz)
      tar -zcf "$DIST/$NAME-${version}-${os}-${arch}.tar.gz" -C $BUILD $binary
      ;;
    zip)
      zip -rq9j "$DIST/$NAME-${version}-${os}-${arch}.zip" "$BUILD/$binary"
    ;;
    *)
      error "invalid package format only 'deb', 'rpm' and 'tgz' are accepted"
  esac
}

while [[ $# -gt 0 ]]; do
  case $1 in
    -a|--arch)
      ARCH="$2"
      if [[ ! $ARCH =~ amd64|arm64 ]]; then
        error "supported architectures are 'amd64' and 'arm64'"
      fi
      shift 
      ;;
    -b|--build)
      BUILD="$2"
      shift
      ;;
    -c|--config)
      CONFIG="$2"
      shift
      ;;
    -d|--dist)
      DIST="$2"
      shift
      ;;
    -o|--os)
      OS="$2"
      if [[ ! $OS =~ linux|darwini|windows ]]; then
        error "supported operating systems are 'linux', 'darwin' and 'windows'"
      fi
      shift
      ;;
    -p|--pkg)
      PKG="$2"
      if [[ ! $PKG =~ deb|rpm|tgz|zip ]]; then
        error "supported package formats are  'deb', 'rpm', 'tgz' and 'zip'"
      fi
      shift
      ;;
    -v|--version)
      VERSION="$2"
      shift
      ;;
    -y|--binary)
      BINARY="$2"
      shift
      ;;
    -*|--*)
      error "Unknown option $1"
      ;;
    *)
      CMD="$1"
      break  # ignore rest of parameters
      ;;
  esac
  shift
done

if [[ ! -e $BUILD ]]; then
   mkdir -p $BUILD
fi

if [[ ! -e $DIST ]]; then
   mkdir -p $DIST
fi


## make all paths absolute
BUILD=$(realpath $BUILD)
DIST=$(realpath $DIST)
CONFIG=$(realpath $CONFIG)

if [[ $CMD =~ build|pack ]]; then
      if [[ -z $OS ]]; then
        error "target operating system is required"
      fi

      if [[ -z $ARCH ]]; then
        error "target architecture is required"
      fi
fi

if [[ $CMD == "pack" ]]; then
    if [[ -z $PKG ]]; then
      error "package format is required"
    fi
fi

if [[ $CMD =~ pack|all ]]; then
   if [[ -z $VERSION ]]; then
      error "version is required"
    fi
fi

case $CMD in
   "build")
      build $OS $ARCH $VERSION $BINARY
      ;;
    "pack")
      package $OS $ARCH $PKG $VERSION $BINARY
      ;;
    "all")
      package linux amd64 deb $VERSION
      package linux amd64 rpm $VERSION
      package linux amd64 tgz $VERSION
      package linux arm64 tgz $VERSION
      package darwin amd64 tgz $VERSION
      package darwin arm64 tgz $VERSION
      package windows amd64 zip $VERSION $NAME-windows-amd64.exe
      ;;
    *)
      error "supported commands are 'build', 'pack' and 'all'"
      ;;
esac
