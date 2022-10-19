# Signalilo

Signalilo is our Alertmanager to Icinga2 bridge implementation. Signalilo acts
on webhooks which it receives from Alertmanager and forwards the alerts in
those webhooks to Icinga2 using https://github.com/vshn/go-icinga2-client.

See [CHANGELOG.md](/CHANGELOG.md) for changelogs of each release version of
Signalilo.

See [DockerHub](https://hub.docker.com/r/vshn/signalilo) or
[Quay.io](https://quay.io/repository/vshn/signalilo) for pre-built
Docker images of Signalilo.

## Usage

Signalilo gets started from the command line and takes its configuration
either as options or as environment variables. Use `signalilo --help` to get a
list of all available configuration parameters.

When started, Signalilo listens to HTTP requests on the following paths:

* `/webhook` Endpoint to accept alerts from Alertmanager.
* `/healthz` returns HTTP 200 with `ok` as its payload as long as the webhook
  serving loop is operational.

## Installation

Helm

    helm install --name signalilo appuio/signalilo

See https://github.com/appuio/charts/tree/master/appuio/signalilo.

Docker

    docker run --name signalilo vshn/signalilo

OpenShift

The Helm chart should work on OpenShift

## Configuration

Mandatory

* `--uuid`/`SIGNALILO_UUID`:
  UUID which identifies the Signalilo instance.
* `--icinga_hostname`/`SIGNALILO_ICINGA_HOSTNAME`:
  Name of the Servicehost in Icinga2.
* `--icinga_url`/`SIGNALILO_ICINGA_URL`:
  URL of the Icinga API. It's possible to specify one or more URLs. 
  The Parameter content will be split on newline character `\n`, e.g. `"http://example.com:5665\nhttp://example2.com:5665"` will configure two masters at `http://example.com:5665` and `http://example2.com:5665`.
  Please keep in mind that the first URL will be the Icinga-Config-Master.
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
* `--icinga_disable_keepalives`/`SIGNALILO_ICINGA_DISABLE_KEEPALIVES`:
  If true, disable http keep-alives with Icinga2 API and will only use
  the connection to the server for a single HTTP request
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
* `--icinga_service_checks_active`/`SIGNALILO_ICINGA_SERVICE_CHECKS_ACTIVE`:
  Use active checks for created icinga services to leverage on Alertmanager resend interval to manage stale checks (
  default: false).
* `--icinga_service_checks_command`/`SIGNALILO_ICINGA_SERVICE_CHECKS_COMMAND`:
  Name of the check command used in Icinga2 service creation (default: 'dummy').
* `--icinga_service_checks_interval`/`SIGNALILO_ICINGA_SERVICE_CHECKS_INTERVAL`:
  Interval (in seconds) to be used for icinga `check_interval` and `retry_interval`.
  This should be set to a multiple of alertmanager `repeat_interval` in case
  active checks are enabled (e.g. `1.1 < icinga_service_checks_interval/repeat_interval < 5`, default: 43200s).
* `--icinga_service_max_check_attempts`/`SIGNALILO_ICINGA_SERVICE_MAX_CHECKS_ATTEMPTS`:
  The maximum number of checks which are executed before changing to a hard state.
* `--icinga_reconnect`/`SIGNALILO_ICINGA_RECONNECT`:
  If it's set, Signalilo to waits for a reconnect instead of switching immediately to another URL.
* `--alertmanager_port`/`SIGNALILO_ALERTMANAGER_PORT`:
  Port on which Signalilo listens to incoming webhooks (default 8888).
* `--alertmanager_bearer_token`/`SIGNALILO_ALERTMANAGER_BEARER_TOKEN`:
  Incoming webhook authentication. Can be either set via `Authorization` header or in the `token` URL query parameter.
* `--alertmanager_tls_cert`/`SIGNALILO_ALERTMANAGER_TLS_CERT`:
  Path of certificate file for TLS-enabled webhook endpoint. Should contain the
  full chain.
* `--alertmanager_tls_key`/`SIGNALILO_ALERTMANAGER_TLS_KEY`:
  Path of private key file for TLS-enabled webhook endpoint. TLS is enabled
  when both TLS_CERT and TLS_KEY are set.
* `--alertmanager_pluginoutput_annotations`/`SIGNALILO_ALERTMANAGER_PLUGINOUTPUT_ANNOTATIONS`:
  The name of an annotation to retrieve the `plugin_output` from. Can be set multiple times in which case the first
  annotation with a value found is used.
* `--alertmanager_pluginoutput_by_states`/`SIGNALILO_ALERTMANAGER_PLUGINOUTPUT_BY_STATES`:
  Enables support for dynamically selecting the Annotation name used for the Plugin Output based on the computed Service
  State.
  See [Plugin Output](#plugin-output) for more details on this option.
* `--alertmanager_custom_severity_levels`/`SIGNALILO_ALERTMANAGER_CUSTOM_SEVERITY_LEVELS`:
  Add or override the default mapping of the `severity` label of the Alert to an Icinga Service State. Use the
  format `label_name=service_state`. The `service_state` can be `0` for OK, `1` for Warning, `2` for Critical, and `3`
  for Unknown. Can be set multiple times and you can also override the default values for the labels `warning`
  and `critical`. The `severity` label is not case-sensitive.

The environment variable names are generated from the command-line flags. The flag is uppercased and all `-` characters
are replaced with `_`. Signalilo uses the newline character `\n` to split flags that are allowed multiple times (
like `SIGNALILO_ALERTMANAGER_PLUGINOUTPUT_ANNOTATIONS`) into an array.

## Integration to Prometheus/Alertmanager.

The `/webhook` accepts alerts in the [format of Alertmanager][webhook_format].
The following Alertmanager configuration is an example taken from a Signalilo
installation on OpenShift.

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

* `severity`: Must be one of `warning` or `critical`, or any values set via the `--alertmanager_custom_severity_levels`
  option.
* `alertname` mapped to `display_name`.

Required annotations:

* `description`: mapped to `notes`.
* `message`: mapped to `plugin_output`.

You can also use the `--alertmanager_pluginoutput_annotations` option to change
the Annotation used for the `plugin_output` as well as the `--alertmanager_pluginoutput_by_states` option.
See [Plugin Output](#plugin-output) for more details.

Optional annotations:

* `runbook_url`: mapped to `notes_url

Infered fields:

* `generatorURL`: mapped to `action_url`

### Plugin Output

By default, Signalilo will use the `message` Annotation to set the `plugin_output` in the Icinga Service.

This can be changed by using the `--alertmanager_pluginoutput_annotations` to select either a
different Annotation or to provide a list of Annotations where the first one with a value will be used.

Alternatively if you enable the `--alertmanager_pluginoutput_by_states` option then Signalilo will
take the Service State name (`ok`, `warning`, `critical`, or `unknown`) and suffix this to the
Annotation name when looking up the Annotation to use for the Plugin Output (for example: `message_ok`).

This allows you to configure multiple Annotations with different values that are then used
with the corresponding Service State to set the Plugin Output.

If an Annotation is not found for that specific Service State then Signalilo will fall back ot just using the Annotation
name as configured.

## Integration with Icinga

### Icinga host

You need to create an Icinga service host which Signalilo can use.
Signalilo is designed to expect that it has full control over one service host in Icinga.
Therefore, you should create a service host for each Signalilo instance which you're running.

Each service host should look as shown below.
You can add additional configurations (such as host variables) as you like.

```
object Host "signalilo_cluster.example.com"  {
  display_name = "Signalilo signalilo_cluster.example.com"
  check_command = "dummy"
  enable_passive_checks = false
  enable_perfdata = false
}
```

### Icinga API user

We recommend that you create an API user per Icinga service host.
This naturally ensures that you create an API user per Signalilo instance, since you should have a service host per
Signalilo instance.
In that case, you can restrict the API user's permissions to only interact with the service host belonging to the
Signalilo instance as shown below.

```
object ApiUser "signalilo_cluster.example.com"  {
  password = "verysecretpassword"
  permissions = [
  {
    permission = "objects/query/*"
    filter = {{ host.name == "signalilo_cluster.example.com" }}
  },
  {
    permission = "objects/create/service"
    filter = {{ host.name == "signalilo_cluster.example.com" }}
  },
  {
    permission = "objects/modify/service"
    filter = {{ host.name == "signalilo_cluster.example.com" }}
  },
  {
    permission = "objects/delete/service"
    filter = {{ host.name == "signalilo_cluster.example.com" }}
  },
  {
    permission = "actions/process-check-result"
    filter = {{ host.name == "signalilo_cluster.example.com" }}
  }, ]
}
```

Note that you don't have to use the same name for the API user as for its associated service host.
However, you have to make sure that you compare `host.name` to the name of the service host for which the API user
should have permissions.

### Garbage Collection

Service objects in Icinga will get garbage collected (aka deleted) on a regular basis, following these rules:

* Service object is in OK state
* Last transition to OK state was more than "keep_for" ago
* UUID of app matches "vars.bridge_uuid"

All state needed for doing garbage collection is stored in Icinga service variables.

### Signalilo Heartbeat

On startup, Signalilo checks if the matching heartbeat service is available in
Icinga, otherwise it exits with a fatal error. During operation, Signalilo
regularly posts its state to the heartbeat service. If no state update was
provided, Icinga automatically marks the check as UNKNOWN.

You need to configure the following service in Icinga:

```
object Service "heartbeat" {
  check_command = "dummy"
  check_interval = 10s

  /* Set the state to CRITICAL (2) if freshness checks fail. */
  vars.dummy_state = 2

  /* Use a runtime function to retrieve the last check time and more details. */
  vars.dummy_text = {{
    var service = get_service(macro("$host.name$"), macro("$service.name$"))
    var lastCheck = DateTime(service.last_check).to_string()

    return "No check results received. Last result time: " + lastCheck
  }}

  /* This must match the name of the host object for the Signalilo instance */
  host_name = "signalilo_cluster.example.com"
}
```

### Custom Variables

All labels and annotations will be mapped to custom variables. Keys of Labels
will be prefixed with `label_` and keys of annotations with `annotation_`.

If the key an annotation or label starts with `icinga_` it will also be added
as custom variable but without any prefix. Since all labels and annotations
will be strings, a type information needs to be provided so that a conversion
can be done accordingly. This is done by adding the type as part of the prefix
(`icinga_<type>_`). Current supported types are `number` and `string`.

Examples:

* `foo` -> `label_foo` or an `anotation_foo`.
* `icinga_string_foo` -> label/annotation named `foo` with value is passed
  as is.
* `icinga_number_bar` -> label/annotation named `bar` with its value is
  converted to an integer number.

In case there is a label and an annotation with the `icinga_<type>` prefix, the
value of the annotation will take precedence in the resulting set of custom
variables.

### Heartbeat Services

Signalilo supports creating heartbeat services in Icinga. This can be used to
map alerts like the `DeadMansSwitch` which comes with `prometheus-operator`
and signals that the whole Prometheus stack is healthy.

In order for Signalilo to treat an alert as a heartbeat, the alert must have
a label `heartbeat`. Signalilo will try to parse the value of that label as a
[Go duration].

If the value is parsed successfully, Signalilo will create an Icinga service
check with active checks enabled and with the check interval set to the
parsed duration plus ten percent. We add ten percent to the parsed duration
to account for network latencies etc., which could otherwise lead to flapping
heartbeat checks.


[Go duration]: https://golang.org/pkg/time/#ParseDuration

[webhook_format]: https://prometheus.io/docs/alerting/configuration/#webhook_config.
