# Tool versions. Each defaults to the latest release; pin one with e.g.
#   docker build --build-arg MAGO_VERSION=1.49.0 docker/
# (or `make docker-build MAGO_VERSION=1.49.0`).
ARG PHP_VERSION=8.3
ARG COMPOSER_VERSION=latest
ARG PHP_CS_FIXER_VERSION=latest
ARG PHPSTAN_VERSION=latest
ARG MAGO_VERSION=latest

# COPY --from can't expand build args, so versioned images are pulled in as
# named stages instead.
FROM composer:${COMPOSER_VERSION} AS composer
FROM ghcr.io/carthage-software/mago:${MAGO_VERSION} AS mago

FROM php:${PHP_VERSION}-cli

# Define build arguments for UID and GID
ARG UID=1000
ARG GID=1000

# Install necessary PHP extensions and their dependencies
RUN apt-get update && apt-get install -y \
	git \
    libicu-dev \
    libxml2-dev \
	libzip-dev \
	unzip \
    && rm -rf /var/lib/apt/lists/*

RUN docker-php-ext-install \
    intl \
    ctype \
    iconv \
    xml \
	zip

ARG PHP_CS_FIXER_VERSION
ARG PHPSTAN_VERSION

# Tools are installed after the extensions so changing a version doesn't
# invalidate the (slow) extension build layers.

# Install Composer
COPY --from=composer /usr/bin/composer /usr/local/bin/composer

# Install mago (statically linked, so the binary from its Alpine image runs here)
COPY --from=mago /usr/local/bin/mago /usr/local/bin/mago

# Download a GitHub release asset: $1 = repo, $2 = tag ("latest" for the
# newest release), $3 = asset name, $4 = destination. curl -f makes an
# unknown version fail the build instead of saving a 404 page. php-cs-fixer
# tags are v-prefixed; its version is accepted with or without the "v".
RUN set -eu; \
    fetch_release() { \
        if [ "$2" = "latest" ]; then url="https://github.com/$1/releases/latest/download/$3"; \
        else url="https://github.com/$1/releases/download/$2/$3"; fi; \
        curl -fsSL "$url" -o "$4" && chmod +x "$4"; \
    }; \
    cs_fixer_tag="$PHP_CS_FIXER_VERSION"; \
    case "$cs_fixer_tag" in latest|v*) ;; *) cs_fixer_tag="v$cs_fixer_tag" ;; esac; \
    fetch_release PHP-CS-Fixer/PHP-CS-Fixer "$cs_fixer_tag" php-cs-fixer.phar /usr/local/bin/php-cs-fixer; \
    fetch_release phpstan/phpstan "$PHPSTAN_VERSION" phpstan.phar /usr/local/bin/phpstan

# Configure git to allow the /app directory as a safe directory
RUN git config --global --add safe.directory /app

# Set the working directory
WORKDIR /app

# Create a non-root user and group using the system user
RUN groupadd -g ${GID} appgroup && useradd -u ${UID} -ms /bin/bash -g appgroup appuser
RUN chown -R appuser:appgroup /app
USER appuser
