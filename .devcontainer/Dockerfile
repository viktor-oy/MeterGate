FROM node:24.2.0-bookworm-slim

ARG COMPOSER_VERSION=2.9.2

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    gnupg \
  # 1. Add official Docker repository
  && install -m 0755 -d /etc/apt/keyrings \
  && curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg \
  && chmod a+r /etc/apt/keyrings/docker.gpg \
  && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian bookworm stable" > /etc/apt/sources.list.d/docker.list \
  # 2. Add official PostgreSQL repository for PG 18
  && curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /etc/apt/keyrings/postgresql.gpg \
  && chmod a+r /etc/apt/keyrings/postgresql.gpg \
  && echo "deb [signed-by=/etc/apt/keyrings/postgresql.gpg] http://apt.postgresql.org/pub/repos/apt bookworm-pgdg main" > /etc/apt/sources.list.d/pgdg.list \
  # 3. Add Ondřej Surý's repository for PHP 8.4
  && curl -fsSL https://packages.sury.org/php/apt.gpg | gpg --dearmor -o /etc/apt/keyrings/sury-php.gpg \
  && chmod a+r /etc/apt/keyrings/sury-php.gpg \
  && echo "deb [signed-by=/etc/apt/keyrings/sury-php.gpg] https://packages.sury.org/php/ bookworm main" > /etc/apt/sources.list.d/php.list \
  # 4. Fetch the new repository lists and install your exact pinned versions
  && apt-get update \
  && apt-get install -y --no-install-recommends \
    bash \
    docker-ce-cli \
    git \
    postgresql-client-18 \
    redis-tools \
    unzip \
    php8.4-cli \
    php8.4-curl \
    php8.4-mbstring \
    php8.4-pgsql \
    php8.4-xml \
    php8.4-zip \
  # Clean up all caches to keep the image slim
  && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://getcomposer.org/download/${COMPOSER_VERSION}/composer.phar" \
  -o /usr/local/bin/composer \
  && chmod +x /usr/local/bin/composer

WORKDIR /workspaces/metergate