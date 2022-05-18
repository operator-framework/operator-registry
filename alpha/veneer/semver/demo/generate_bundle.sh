#!/bin/bash 
set -e

package=${package:-testoperator}
version=${version:-0.0.0}
channels=${channels:-stable}
default=${default:-stable}
name=${suffix:-$package}.v$version
tag=${tag:-$name}
targetrepo=${targetrepo:-quay.io/ankitathomas/olm}
image="$targetrepo:$tag"
force=${false:false}

if [ -n "$replaces" ]; then
	replaces="$package.v$replaces"
fi

if [ -e $name ]; then
  if $force; then
	echo "overwriting operator directory $name"
	rm -rf $name
  else
	echo "$name exists, exiting as force=false"
	exit 1
  fi
fi


#opm doesn't overwrite this on its own yet
if [ -e bundle.Dockerfile ]; then
  if $force; then
	echo "overwriting bundle.Dockerfile"
	rm -rf bundle.Dockerfile
  else
	echo "bundle.Dockerfile exists, exiting as force=false"
        exit 1
   fi
fi

mkdir -p $name/manifests

cat <<EOF > $name/manifests/csv.yaml
#! parse-kind: ClusterServiceVersion
apiVersion: operators.coreos.com/v1alpha1
kind: ClusterServiceVersion
metadata:
  name: $name
  namespace: placeholder
  annotations:
    olm.skipRange: "$skiprange"
spec:
  displayName: $package
  version: $version
  skips: [ $skips ]
  replaces: "$replaces"
  install:
    strategy: deployment
    spec:
      deployments:
      - name: $package
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: $package
          template:
            metadata:
              name: $package
              labels:
                app: $package
                version: $version
            spec:
              serviceAccountName: $package
              containers:
              - name: multichannel
                command: ['sh', '-c', 'echo Container 1 is Running ; sleep 3600']
                image: busybox
                imagePullPolicy: "IfNotPresent"
                resources:
                  requests:
                    cpu: "1m"
                    memory: "32Mi"
                  limits:
                    cpu: "1"
                    memory: "128Mi"
  installModes:
  - supported: true
    type: OwnNamespace
EOF

./bin/opm alpha bundle build -c $channels -e $default -d $name/manifests -p $package -o -t "$image"
docker push "$image"

rm -rf $name bundle.Dockerfile
