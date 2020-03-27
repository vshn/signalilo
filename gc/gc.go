/*
 * Authors:
 * Simon Gerber <simon.gerber@vshn.ch>
 *
 * License:
 * Copyright (c) 2019, VSHN AG, <info@vshn.ch>
 * Licensed under "BSD 3-Clause". See LICENSE file.
 */

package gc

import (
	"fmt"
	"time"

	"github.com/vshn/signalilo/config"
	"github.com/vshn/go-icinga2-client/icinga2"
)

// extractDowntime searches the provided downtime array for a downtime for
// service with name svcName.
func extractDowntime(downtimes []icinga2.Downtime, svcName string) (icinga2.Downtime, bool) {
	for _, dt := range downtimes {
		if dt.Service == svcName {
			return dt, true
		}
	}
	return icinga2.Downtime{}, false
}

// collectService cleans up a single service that is managed by this Signalilo
func collectService(svc icinga2.Service, c config.Configuration, downtimes []icinga2.Downtime) error {
	l := c.GetLogger()
	icinga := c.GetIcingaClient()

	_, heartbeat := svc.Vars["label_heartbeat"]
	_, downtimed := extractDowntime(downtimes, svc.Name)
	if heartbeat && !downtimed {
		l.V(2).Infof(fmt.Sprintf("[Collect] Skipping heartbeat %v: not downtimed", svc.Name))
		return nil
	} else if svc.State > 0 && !heartbeat {
		l.V(2).Infof(fmt.Sprintf("[Collect] Skipping service %v: state=%v, downtimed=%v",
			svc.Name, svc.State, downtimed))
		return nil
	}

	keepForNs := int64(svc.Vars["keep_for"].(float64))
	keepFor := time.Duration(keepForNs)
	lastChangeUnixNs := int64(svc.LastStateChange * 1e9)
	lastChange := time.Unix(0, lastChangeUnixNs)
	serviceAge := time.Since(lastChange)
	if serviceAge >= keepFor {
		l.V(2).Infof("[Collect] Deleting service %v: keep_for = %v; age = %v", svc.Name, keepFor, serviceAge)
		err := icinga.DeleteService(svc.FullName())
		if err != nil {
			l.Errorf(fmt.Sprintf("Error while deleting service: %v", err))
		}
	} else {
		l.V(2).Infof("[Collect] Skipping service %v: keep_for = %v; age = %v", svc.Name, keepFor, serviceAge)
	}
	return nil
}

// Collect runs a garbage collection cycle to clean up any old
// Signalilo-managed service objects
func Collect(ts time.Time, c config.Configuration) error {
	l := c.GetLogger()
	l.Infof("[Collect] Running garbage collection at ts=%v", ts)
	// Get all signalilo services
	icinga := c.GetIcingaClient()
	hostname := c.GetConfig().HostName
	services, err := icinga.ListServices(fmt.Sprintf("host=%v", hostname))
	if err != nil {
		l.Errorf(fmt.Sprintf("[Collect] Error while listing services: %v", err))
		return err
	}
	l.V(2).Infof("[Collect] Found %v services with host = %v", len(services), hostname)
	downtimes, err := icinga.ListDowntimes(fmt.Sprintf("host=%v", hostname))
	if err != nil {
		l.Errorf(fmt.Sprintf("[Collect] Error while listing downtimes: %v", err))
		return err
	}
	l.V(2).Infof("[Collect] Found %v downtimes with host = %v", len(downtimes), hostname)
	// Iterate through services, finding ones that are managed by this
	// Signalilo and delete services which have transitioned to OK longer
	// than keep_for ago
	for _, svc := range services {
		if svc.Vars["bridge_uuid"] == c.GetConfig().UUID {
			l.Infof("[Collect] Found service %v with our bridge UUID", svc.Name)
			err = collectService(svc, c, downtimes)
			if err != nil {
				l.Errorf(fmt.Sprintf("[Collect] Error garbage-collecting service: %v", err))
			}
		}
	}
	l.Infof("[Collect] Garbage collection completed in %v", time.Since(ts))
	return nil
}
