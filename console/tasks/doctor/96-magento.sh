#!/usr/bin/env bash
# Magento installation
source "$HELPERS_DIR"/doctor.sh
doctor_requires_project

lock="${MAGENTO_DIR:-.}/composer.lock"

if [ ! -f "$lock" ]; then
    doctor_warning "No composer.lock found in ${MAGENTO_DIR:-.}" \
        "$COMMAND_BIN_NAME composer install"
    exit 0
fi

if [ -z "${HM_MAGENTO:-}" ]; then
    doctor_warning "composer.lock has no Magento package" \
        "Check that this is a Magento project"
    exit 0
fi

if [ ! -f "${MAGENTO_DIR:-.}/app/etc/env.php" ]; then
    doctor_warning "Magento $HM_MAGENTO is not installed yet (no app/etc/env.php)" \
        "$COMMAND_BIN_NAME install"
    exit 0
fi

doctor_ok "Magento $HM_MAGENTO"
