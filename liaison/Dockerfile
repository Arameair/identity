# identity-liaison container. Standard library only, so the image stays small.
ARG BASE=python:3.12-alpine
FROM ${BASE}
ARG PIP_FLAGS=""

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    HOME=/tmp \
    IDENTITY_LIAISON_CONFIG=/config/config.toml \
    GITHUB_TOKEN_FILE=/run/secrets/github_token

WORKDIR /src
COPY pyproject.toml README.md ./
COPY identity_liaison ./identity_liaison
RUN python3 -m pip install --no-cache-dir ${PIP_FLAGS} . && rm -rf /src

# Runs as an unprivileged user; compose passes your own UID so ./config stays yours.
USER 1000:1000
WORKDIR /tmp
ENTRYPOINT ["identity-liaison"]
CMD ["run"]
