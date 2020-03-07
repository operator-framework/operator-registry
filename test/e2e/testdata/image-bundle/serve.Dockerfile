# docker build -t init-operator-manifest .
FROM busybox

COPY opm /bin
