# Signalilo

Signalilo is our Alertmanager to Icinga2 bridge implementation. Signalilo acts
on webhooks which it receives from Alertmanager and forwards the alerts in
those webhooks to Icinga2 using https://github.com/vshn/go-icinga2-client.

## Usage

Signalilo gets started from the command line and takes its configuration
eighter as options or as environment variables. Use `signalilo --help` to get a
list of all available configuration parameters.

When started, Signalilo listens to HTTP requests on the following paths:

* `/webhook` Endpoint to accepts alerts from Alertmanager.
* `/healthz` returns HTTP 200 with `ok` as its payload as long as the webhook
  serving loop is operational.

## Installation

Helm

    helm install --name signalilo appuio/signalilo

See https://github.com/appuio/charts/tree/master/signalilo.

Docker

    docker run --name signalilo vshn/signalilo

OpenShift

See https://git.vshn.net/appuio/mungg-cluster-monitoring.

## Configuration

Mandatory

* `--uuid`/`SIGNALILO_UUID`:
  UUID which identifies the Signalilo instance.
* `--icinga_hostname`/`SIGNALILO_ICINGA_HOSTNAME`:
  Name of the Servicehost in Icinga2.
* `--icinga_url`/`SIGNALILO_ICINGA_URL`:
  URL of the Icinga API.
* `--icinga_username`/`SIGNALILO_ICINGA_USERNAME`:
  Authentication against Icinga2 API.
* `--icinga_password`/`SIGNALILO_ICINGA_PASSWORD`:
  Authentication against Icinga2 API.

Optional

* `--loglevel`/`SIGNALILO_LOG_LEVEL`:
  Integer to control verbosity of logging (default: 2).
* `--icinga_insecure_tls`/`SIGNALILO_ICINGA_INSECURE_TLS`:
  If true, disable strict TLS checking of Icinga2 API SSL certificate
  (default: false).
* `--icinga_debug`/`SIGNALILO_ICINGA_DEBUG`:
  If true, enable debugging mode in Icinga client (default: false).
* `--icinga_gc_interval`/`SIGNALILO_ICINGA_GC_INTERVAL`:
  Interval to run Garbage collection of recovered alerts in Icinga
  (default 15m).
* `--icinga_heartbeat_interval`/`SIGNALILO_ICINGA_HEARTBEAT_INTERVAL`:
  Interval to send heartbeat to Icinga (default 60s).
* `--icinga_keep_for`/`SIGNALILO_ICINGA_KEEP_FOR`:
  How long to keep Icinga2 services around after they transition to state OK
  (default 168h).
* `--icinga_ca`/`SIGNALILO_ICINGA_CA`:
  A PEM string of the trusted CA certificate for the Icinga2 API certificate.
* `--alertmanager_port`/`SIGNALILO_ALERTMANAGER_PORT`:
  Port on which Signalilo listens to incoming webhooks (default 8888).
* `--alertmanager_bearer_token`/`SIGNALILO_ALERTMANAGER_BEARER_TOKEN`:
  Incoming webhook authentication.
* `--alertmanager_tls_cert`/`SIGNALILO_ALERTMANAGER_TLS_CERT`:
  Path of certificate file for TLS-enabled webhook endpoint. Should contain the
  full chain.
* `--alertmanager_tls_key`/`SIGNALILO_ALERTMANAGER_TLS_KEY`:
  Path of private key file for TLS-enabled webhook endpoint. TLS is enabled
  when both TLS_CERT and TLS_KEY are set.

## Integration to Prometheus/Alertmanager.

The `/webhook` accepts alerts in the [format of Alertmanager][webhook_format].
The following Alertmanager is an example taken from a Signalilo installation on
OpenShift.

    global:
      resolve_timeout: 5m
    route:
      group_wait: 30s
      group_interval: 5m
      repeat_interval: 12h
      receiver: default
      routes:
      - match:
          alertname: DeadMansSwitch
        repeat_interval: 5m
        receiver: deadmansswitch
    receivers:
    - name: default
      webhook_configs:
      - send_resolved: true
        http_config:
          bearer_token: "*****"
        url: http://signalilo.appuio-monitoring/webhook
    - name: deadmansswitch

Signalilo requires a set of information to be part of an alert. Without this
information, the check generated in Icinga will be lacking.

Required labels:

* `severity`: Must be one of `WARNING` or `CRITICAL`.

Required annotations:

* `summary` mapped to `display_name`.
* `description`: mapped to `notes.
* `message`: mapped to `plugin_output`.

## Integration with Icinga

### Garbage Collection

Service objects in Icinga will get garbage collected (aka deleted) on a regular basis, following these rules:

* Service object is in OK state
* Last transition to OK state was more than "keep_for" ago
* UUID of app matches "vars.bridge_uuid"

All state needed for doing garbage collection is stored in Icinga service variables.

### Heartbeat

On startup of the app the app checks if the matching heartbeat service is
available in Icinga, otherwise fail and exit with a fatal error. On a regular
interval the app posts it's state there. If no state update was provided,
Icinga automatically marks the check as UNKNOWN.  See [Icinga2 passive
checks][passive_checks] for a description of how the service object needs to be
configured.

[passive_checks]: https://wiki.vshn.net/display/VT/Icinga2+passive+checks
[webhook_format]: https://prometheus.io/docs/alerting/configuration/#webhook_config.
