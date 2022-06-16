vcl 4.1;

import std;

backend default {
    .host = "nginx";
    .port = "8080";
    .first_byte_timeout = 600s;
}

# Add hostnames, IP addresses and subnets that are allowed to purge content
acl purge {
    "phpfpm";
    "hitch";
    "nginx";
    "localhost";
    "127.0.0.1";
    "0.0.0.0";
    "::1";
}

sub vcl_recv {
    # Remove empty query string parameters
    # e.g.: www.example.com/index.html?
    if (req.url ~ "\?$") {
        set req.url = regsub(req.url, "\?$", "");
    }

    # Remove port number from host header
    set req.http.Host = regsub(req.http.Host, ":[0-9]+", "");

    # Sorts query string parameters alphabetically for cache normalization purposes
    set req.url = std.querysort(req.url);

    # Remove the proxy header to mitigate the httpoxy vulnerability
    # See https://httpoxy.org/
    unset req.http.proxy;

    # Add X-Forwarded-Proto header when using https (Custom: Forcing to use HTTPS)
    set req.http.X-Forwarded-Proto = "https";

    # Reduce grace to 300s if the backend is healthy
    # In case of an unhealthy backend, the original grace is used
    if (std.healthy(req.backend_hint)) {
        set req.grace = 300s;
    }

    # Skip varnish if custom command is launched
    #return(pass); #skip-varnish

    # Purge logic to remove objects from the cache
    # Tailored to Magento's cache invalidation mechanism
    if (req.method == "PURGE") {
        if (client.ip !~ purge) {
            return (synth(405, "Method not allowed"));
        }
        if (!req.http.X-Magento-Tags-Pattern && !req.http.X-Pool) {
            return (synth(400, "X-Magento-Tags-Pattern or X-Pool header required"));
        }
        if (req.http.X-Magento-Tags-Pattern) {
          ban("obj.http.X-Magento-Tags ~ " + req.http.X-Magento-Tags-Pattern);
        }
        if (req.http.X-Pool) {
          ban("obj.http.X-Pool ~ " + req.http.X-Pool);
        }
        return (synth(200, "Purged"));
    }

    # Only handle relevant HTTP request methods
    if (req.method != "GET" &&
        req.method != "HEAD" &&
        req.method != "PUT" &&
        req.method != "POST" &&
        req.method != "PATCH" &&
        req.method != "TRACE" &&
        req.method != "OPTIONS" &&
        req.method != "DELETE") {
          return (pipe);
    }

    # Only cache GET and HEAD requests
    if (req.method != "GET" && req.method != "HEAD") {
        return (pass);
    }

    # Don't cache the checkout page
    if (req.url ~ "/checkout") {
        return (pass);
    }

    # Don't cache the health check page
    if (req.url ~ "^/(pub/)?(health_check.php)$") {
        return (pass);
    }

    # Collapse multiple cookie headers into one
    std.collect(req.http.Cookie);

    # Remove tracking query string parameters used by analytics tools
    if (req.url ~ "(\?|&)(gclid|cx|ie|cof|siteurl|zanpid|origin|fbclid|mc_[a-z]+|utm_[a-z]+|_bta_[a-z]+)=") {
        set req.url = regsuball(req.url, "(gclid|cx|ie|cof|siteurl|zanpid|origin|fbclid|mc_[a-z]+|utm_[a-z]+|_bta_[a-z]+)=[-_A-z0-9+()%.]+&?", "");
        set req.url = regsub(req.url, "[?|&]+$", "");
    }

    # Don't cache the authenticated GraphQL requests
    if (req.url ~ "/graphql" && req.http.Authorization ~ "^Bearer") {
        return (pass);
    }

    return (hash);
}

sub vcl_hash {
    # Add a cache variation based on the X-Magento-Vary cookie
    if (req.http.cookie ~ "X-Magento-Vary=") {
        hash_data(regsub(req.http.cookie, "^.*?X-Magento-Vary=([^;]+);*.*$", "\1"));
    } else {
        hash_data("");
    }

    # Create cache variations depending on the request protocol
    hash_data(req.http.X-Forwarded-Proto);

    # Create store and currency cache variations for GraphQL requests
    if (req.url ~ "/graphql") {
        hash_data(req.http.Store);
        hash_data(req.http.Content-Currency);
    }
}

sub vcl_backend_response {
	# Serve stale content for three days after object expiration
	# Perform asynchronous revalidation while stale content is served
    set beresp.grace = 3d;

    # All text-based content can be parsed as ESI
    if (beresp.http.content-type ~ "text") {
        set beresp.do_esi = true;
    }

    # Allow GZIP compression on all JavaScript files and all text-based content
    if (bereq.url ~ "\.js$" || beresp.http.content-type ~ "text") {
        set beresp.do_gzip = true;
    }

    # Add debug headers
    if (beresp.http.X-Magento-Debug) {
        set beresp.http.X-Magento-Cache-Control = beresp.http.Cache-Control;
    }

    # Only cache HTTP 200 & HTTP 404 responses
    if (beresp.status != 200 && beresp.status != 404) {
        set beresp.ttl = 120s;
        set beresp.uncacheable = true;
        return (deliver);
    # Don't cache private responses
    } elsif (beresp.http.Cache-Control ~ "private") {
        set beresp.uncacheable = true;
        set beresp.ttl = 86400s;
        return (deliver);
    }

    # Remove the Set-Cookie header for cacheable content
    # Only for HTTP GET & HTTP HEAD requests
    if (beresp.ttl > 0s && (bereq.method == "GET" || bereq.method == "HEAD")) {
        unset beresp.http.set-cookie;
    }

    # Don't cache content with a negative TTL
    # Don't cache content for no-cache or no-store content
    # Don't cache content where all headers are varied
    if (beresp.ttl <= 0s ||
       beresp.http.Surrogate-control ~ "no-store" ||
       (!beresp.http.Surrogate-Control &&
       beresp.http.Cache-Control ~ "no-cache|no-store") ||
       beresp.http.Vary == "*") {
        set beresp.ttl = 120s;
        set beresp.uncacheable = true;
    }

    return (deliver);
}

sub vcl_deliver {
    # Add debug headers
    if (resp.http.X-Magento-Debug) {
        if (obj.uncacheable) {
            set resp.http.X-Magento-Cache-Debug = "UNCACHEABLE";
        } else if (obj.hits) {
            set resp.http.X-Magento-Cache-Debug = "HIT";
            set resp.http.Grace = req.http.grace;
        } else {
            set resp.http.X-Magento-Cache-Debug = "MISS";
        }
    } else {
        unset resp.http.Age;
    }

    # Don't let browser cache non-static files
    if (resp.http.Cache-Control !~ "private" && req.url !~ "^/(pub/)?(media|static)/") {
        set resp.http.Pragma = "no-cache";
        set resp.http.Expires = "-1";
        set resp.http.Cache-Control = "no-store, no-cache, must-revalidate, max-age=0";
    }

    # Cleanup headers
    unset resp.http.X-Magento-Debug;
    unset resp.http.X-Magento-Tags;
    unset resp.http.X-Powered-By;
    unset resp.http.Server;
    unset resp.http.X-Varnish;
    unset resp.http.Via;
    unset resp.http.Link;
}