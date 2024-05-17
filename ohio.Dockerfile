FROM quay.io/operatorhubio/catalog:latest

COPY opm /bin/opm

ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs", "--cache-dir=/tmp/cache"]
