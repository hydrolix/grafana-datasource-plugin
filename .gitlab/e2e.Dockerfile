ARG GRAFANA_IMAGE=grafana-enterprise
ARG GRAFANA_VERSION=latest

FROM grafana/${GRAFANA_IMAGE}:${GRAFANA_VERSION}

ARG PLUGIN_NAME=hydrolix-datasource

ENV GF_AUTH_ANONYMOUS_ORG_ROLE=Admin
ENV GF_AUTH_ANONYMOUS_ENABLED=true
ENV GF_AUTH_BASIC_ENABLED=false
ENV GF_DEFAULT_APP_MODE=development
ENV GF_PATHS_HOME=/usr/share/grafana
ENV GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=$PLUGIN_NAME

USER root
WORKDIR $GF_PATHS_HOME

COPY dist /var/lib/grafana/plugins/$PLUGIN_NAME
COPY provisioning /etc/grafana/provisioning

RUN sed -i 's|</body>|<script src="http://localhost:35729/livereload.js"></script></body>|g' /usr/share/grafana/public/views/index.html

HEALTHCHECK CMD curl -f http://localhost:3000 || exit 1
ENTRYPOINT ["/run.sh"]
