#!/usr/bin/env bash
# Magento installation
source "$HELPERS_DIR"/doctor.sh
source "$TASKS_DIR"/set_environment_labels.sh
doctor_requires_project

# Resolved here rather than read from the environment: the expensive labels are only
# exported for the commands that create containers, and an empty variable would be
# misread as "this project has no Magento"
magento_version="${HM_MAGENTO:-$(hm_magento_version)}"

lock="${MAGENTO_DIR:-.}/composer.lock"

if [ ! -f "$lock" ]; then
    doctor_warning "No composer.lock found in ${MAGENTO_DIR:-.}" \
        "$COMMAND_BIN_NAME composer install"
    exit 0
fi

if [ -z "$magento_version" ]; then
    doctor_warning "composer.lock has no Magento package" \
        "Check that this is a Magento project"
    exit 0
fi

if [ ! -f "${MAGENTO_DIR:-.}/app/etc/env.php" ]; then
    doctor_warning "Magento $magento_version is not installed yet (no app/etc/env.php)" \
        "$COMMAND_BIN_NAME install"
    exit 0
fi

doctor_ok "Magento $magento_version"
