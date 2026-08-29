# install

Runs Magento's `setup:install` against this environment, with the database, cache, session and
queue already wired to the containers.

```bash
hm install                  # asks the install settings
hm install --use-default    # uses the saved settings without asking (-u)
```

Usually you do not run it directly: `hm setup` calls it when you choose to install rather than
import a dump.

## Bootstrapping without being asked anything

```bash
hm setup --yes --clean-install --domain=shop.test
hm setup --yes --db-dump=./dump.sql
```

`hm setup` asks four things. Three have defaults — the project name and the domain come from the
directory, the root directory is the current one — so `--yes` answers them. The fourth, *import a
dump or install Magento*, has no safe default: choosing wrong either wipes a database or spends
twenty minutes installing something nobody wanted, so it refuses under `--yes` unless the answer
was given on the command line.

`--clean-install` and `--db-dump` are Warden's names for `--install` and `--dump`. Both spellings
work, in both the `--option=value` and `--option value` forms, and so do the short `-i` and `-D`.

**A dump that is not there stops the command**, with the usage exit code and the path it could
not find. It used to warn and carry on into the question, which is how an automated bootstrap
hangs instead of failing.

## The admin account

The password is **generated** and shown once, at the end:

```console
Your environment is ready

  storefront   https://project.local/
  admin        https://project.local/admin

  user         hiberus
  password     Kf3nQpX7mTvL2aBcR8d4

  This password is not stored anywhere. Save it now.
```

It is 20 alphanumeric characters — no symbols, because the password travels as an argument through
`docker compose exec` and every symbol is a chance for a layer of quoting to break it. Twenty
alphanumerics carry more entropy than twelve with symbols, and neither is meant to be typed.

**It is not written down.** `data/config.json` lives in the tool's directory, is shared by every
project, and records what you answer — a password written there becomes the next project's default
and sits in plain text on disk.

If you want a fixed password instead, set it in `data/config.json` and it is respected.

### If you lose it

There is nothing to recover; create the user again with the same name and it updates the password:

```bash
hm magento admin:user:create \
  --admin-user=hiberus --admin-password='NewPassword123' \
  --admin-email=you@example.com --admin-firstname=Name --admin-lastname=Surname
```

## The second factor

Magento 2.4 requires a second factor to reach the admin panel. The install registers one and draws
its code so you can enrol straight away:

```
Scan this with your authenticator app:

  █▀▀▀▀▀█ ▄ ▄ █▄█▄▄  ▀▄▀▄█▀ █▀▀▀▀▀█
  █ ███ █ ▄▄▄ █▄▀▄█▄  ▀█ ▀█ █ ███ █
  ...

  account    project.local:hiberus
  key        BQNXE43BRZILRHREAQYNRED6G2KWOZA2
  type       time based (TOTP)

  uri        otpauth://totp/project.local:hiberus?secret=…&issuer=project.local
```

The code is drawn by `endroid/qr-code`, which Magento already ships for the two-factor module's own
admin screen: nothing to install, and the same result on every machine.

**The key is shown whether or not the code is.** Scanning is not always possible — a phone that is
not next to the terminal, a shared screen, an authenticator without a camera — and every app offers
to enter the key by hand. Those three lines are exactly what manual entry asks for: the account,
the key, and that it is time based. Magento's own two-factor screen shows both for the same reason.

The `uri` line is the same thing in one string, for pasting into an app that accepts it.

### If the module is disabled

Some projects disable `Magento_TwoFactorAuth` for local development. The install says so and moves
on, without enabling anything:

```
Two factor authentication is disabled in this project, so none was set up.
```

Enabling it later is a decision with consequences for how everyone on the project logs in, so it
is left to you:

```bash
hm magento module:enable Magento_TwoFactorAuth
hm magento setup:upgrade
```

Then register the factor for your user:

```bash
hm magento security:tfa:google:set-secret <user> <base32-secret>
```
