# docker build -t bundle-image .
FROM fedora

COPY manifests /manifests
COPY metadata /metadata
