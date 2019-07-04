# Signalilo

Signalilo is our Alertmanager to Icinga2 bridge implementation. Signalilo acts
on webhooks which it receives from Alertmanager and forwards the alerts in
those webhooks to Icinga2 using https://github.com/vshn/go-icinga2-client.
