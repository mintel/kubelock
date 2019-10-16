FROM nginxdemos/hello:latest

COPY --from=mintel/kubelock:latest /usr/local/bin/kubelock /usr/local/bin/
COPY db-migrations.sh /
