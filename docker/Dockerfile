FROM php:8.3-cli

# Define build arguments for UID and GID
ARG UID=1000
ARG GID=1000

# Install Composer
COPY --from=composer:latest /usr/bin/composer /usr/local/bin/composer

# Install php-cs-fixer
RUN curl -L https://cs.symfony.com/download/php-cs-fixer-v3.phar -o /usr/local/bin/php-cs-fixer && chmod +x /usr/local/bin/php-cs-fixer

# Install phpstan
COPY --from=ghcr.io/phpstan/phpstan:latest-php8.3 /composer/vendor/phpstan/phpstan/phpstan.phar /usr/local/bin/phpstan

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

# Configure git to allow the /app directory as a safe directory
RUN git config --global --add safe.directory /app

# Set the working directory
WORKDIR /app

# Create a non-root user and group using the system user
RUN groupadd -g ${GID} appgroup && useradd -u ${UID} -ms /bin/bash -g appgroup appuser
RUN chown -R appuser:appgroup /app
USER appuser

