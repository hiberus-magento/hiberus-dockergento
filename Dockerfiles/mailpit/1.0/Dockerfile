# Mailpit: the maintained replacement for Mailhog.
#
# Built from the project's official image rather than compiled from source the way Mailhog is
# here: Mailpit publishes maintained multi-architecture images, and rebuilding them would only
# add a step that can break.
#
# The ports are Mailhog's — 1025 for SMTP, 8025 for the web interface — which is what lets a
# project swap one for the other without touching Magento's configuration.
FROM axllent/mailpit:v1.31.0

LABEL maintainer="ddelgado@hiberus.com"
LABEL description="Mail catcher for local Magento environments"

# A local Magento sends mail with made-up credentials over a plain connection: accepting them is
# the point of a catcher, and refusing them would only produce mail nobody can find.
ENV MP_SMTP_AUTH_ACCEPT_ANY=1
ENV MP_SMTP_AUTH_ALLOW_INSECURE=1

EXPOSE 1025 8025
