#!/usr/bin/env bash

#
# The admin account a fresh install leaves behind.
#
# Two things used to be left to the person installing: a password that was the same for everybody
# and was written to a file shared by every project, and the second factor Magento 2.4 requires to
# reach the panel at all — whose only practical workaround was disabling the module.
#
# Everything here uses what is already installed. The second factor is registered with the
# module's own command, and the QR is drawn by `endroid/qr-code`, which ships in every Magento's
# vendor directory: no dependency on the host, and the same result on anybody's machine.
#

#
# A password Magento accepts, in HM_ADMIN_PASSWORD.
#
# Twenty characters of [A-Za-z0-9]. No symbols on purpose: this travels as an argument to
# `setup:install` through `docker compose exec`, and every symbol is a chance for one of those
# layers of quoting to break it. Twenty alphanumerics carry more entropy than twelve with symbols,
# and neither is going to be typed by hand.
#
hm_generate_admin_password() {
    local password
    password=$(LC_ALL=C tr -dc 'A-Za-z0-9' < /dev/urandom 2>/dev/null | head -c 18)

    # Magento requires letters *and* digits. Appending one of each makes that certain rather than
    # merely likely.
    local letter digit
    letter=$(LC_ALL=C tr -dc 'A-Za-z' < /dev/urandom 2>/dev/null | head -c 1)
    digit=$(LC_ALL=C tr -dc '0-9' < /dev/urandom 2>/dev/null | head -c 1)

    HM_ADMIN_PASSWORD="${password}${letter}${digit}"
}

#
# A TOTP secret, in HM_ADMIN_OTP_SECRET.
#
# Base32 is what authenticator apps expect, and what Magento's own generator produces. Thirty-two
# characters of it is 160 bits.
#
hm_generate_otp_secret() {
    HM_ADMIN_OTP_SECRET=$(LC_ALL=C tr -dc 'A-Z2-7' < /dev/urandom 2>/dev/null | head -c 32)
}

#
# Is the two factor module active in this project?
#
hm_two_factor_enabled() {
    "$COMMANDS_DIR"/magento.sh module:status Magento_TwoFactorAuth 2>/dev/null |
        grep -qi "module is enabled"
}

#
# Register the second factor for a user, and leave the registration URI in HM_ADMIN_OTP_URI
#
hm_register_second_factor() {
    local user="$1"
    local issuer="${DOMAIN:-magento}"

    hm_generate_otp_secret

    if ! "$COMMANDS_DIR"/magento.sh security:tfa:google:set-secret "$user" "$HM_ADMIN_OTP_SECRET" \
        >/dev/null 2>&1; then
        HM_ADMIN_OTP_URI=""
        return 1
    fi

    HM_ADMIN_OTP_URI="otpauth://totp/${issuer}:${user}?secret=${HM_ADMIN_OTP_SECRET}&issuer=${issuer}"
}

#
# Draw a URI as a QR code, using the project's own library inside the container.
#
# Magento ships endroid/qr-code for the two factor module's own admin screen, and it comes with a
# writer for terminals. Nothing to install, and it looks the same on every machine.
#
hm_print_qr() {
    local uri="$1"

    $DOCKER_COMPOSE exec -T phpfpm php -r '
        $uri = $argv[1] ?? "";
        if (!is_file("vendor/autoload.php")) { exit(1); }
        require "vendor/autoload.php";
        $writer = "Endroid\\QrCode\\Writer\\ConsoleWriter";
        $code   = "Endroid\\QrCode\\QrCode";
        if (!class_exists($writer) || !class_exists($code)) { exit(1); }
        $qr = method_exists($code, "create") ? $code::create($uri) : new $code($uri);
        echo (new $writer())->write($qr)->getString();
    ' -- "$uri" 2>/dev/null
}

#
# What is needed to log in for the first time, in one block at the end.
#
# The password appears once, here, where no further command output can push it off the screen.
#
hm_print_admin_summary() {
    local user="$1"
    local password="$2"

    printf '\n'
    print_heading "Your environment is ready\n"
    printf '\n'

    printf '  %-12s ' "storefront"
    print_link "https://${DOMAIN}/\n"
    printf '  %-12s ' "admin"
    print_link "https://${DOMAIN}/admin\n"
    printf '\n'
    printf '  %-12s %s\n' "user" "$user"
    printf '  %-12s %s\n' "password" "$password"
    printf '\n'
    print_warning "This password is not stored anywhere. Save it now.\n"

    if [ -n "${HM_ADMIN_OTP_URI:-}" ]; then
        printf '\n'
        print_info "Scan this with your authenticator app:\n\n"

        if ! hm_print_qr "$HM_ADMIN_OTP_URI"; then
            print_warning "  Could not draw the code. Use this instead:\n"
        fi

        printf '\n  %s\n' "$HM_ADMIN_OTP_URI"
    fi

    printf '\n'
}
