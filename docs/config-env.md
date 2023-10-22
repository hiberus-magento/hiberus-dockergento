# hm config-env

Set specific configurations in `core_config_data`.

## Usage

```bash
hm config-env
```

## Description

The `config-env` serves to set multiples useful settings for our local environment. You can add or remove multiple configurations in file '<project>/config/docker/config-env.json'. The initials configurations are:

```json
{
    "config_set": [
        {
            "path": "web/unsecure/base_url",
            "value": "URL"
        }, {
            "path": "web/secure/base_url",
            "value": "URL"
        }, {
            "path": "web/cookie/cookie_domain",
            "value": "DOMAIN"
        }, {
            "path": "catalog/search/elasticsearch7_server_hostname",
            "value": "search"
        }, {
            "path": "catalog/search/elasticsearch7_server_port",
            "value": "9200"
        }, {
            "path": "web/secure/offloader_header",
            "value": "X-Forwarded-Proto"
        }, {
            "path": "admin/security/admin_account_sharing",
            "value": "1"
        }, {
            "path": "admin/security/session_lifetime",
            "value": "31536000"
        }, {
            "path": "admin/security/password_lifetime",
            "value": "null"
        }, {
            "path": "admin/security/password_is_forced",
            "value": "0"
        }
    ]
}
```
You can add configurations for specific scope
```json
        {
            "scope": "store",
            "scope-code": "admin",
            "path": "web/unsecure/base_url",
            "value": "URL"
        }, {
            "scope": "store",
            "scope-code": "admin",
            "path": "web/secure/base_url",
            "value": "URL"
        }, 
```