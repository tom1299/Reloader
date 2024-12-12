package handler

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

type DelayedUpdate struct {
	id         string
	item       runtime.Object
	rollTime   time.Time
	updateFunc func(kube.Clients, string, runtime.Object) error
	namespace  string
}

var DelayedUpdates = make(map[string]DelayedUpdate)

func HandleDelayedUpdate(resourceName string, delayedRoll DelayedUpdate, clients kube.Clients, upgradeFuncs callbacks.RollingUpgradeFuncs, config util.Config) {
	DelayedUpdates[resourceName] = delayedRoll

	go func(resourceName string) {
		<-time.After(10 * time.Second)
		delayedRoll := DelayedUpdates[resourceName]
		logrus.Infof("Executing delayed update for '%s' of type '%s' in namespace '%s'", resourceName, upgradeFuncs.ResourceType, config.Namespace)
		err := delayedRoll.updateFunc(clients, delayedRoll.namespace, delayedRoll.item)
		if err != nil {
			logrus.Errorf("Delayed update for '%s' of type '%s' in namespace '%s' failed with error %v", resourceName, upgradeFuncs.ResourceType, config.Namespace, err)
		} else {
			logrus.Infof("Delayed update for '%s' of type '%s' in namespace '%s' was successful", resourceName, upgradeFuncs.ResourceType, config.Namespace)
		}
		delete(DelayedUpdates, resourceName)
	}(resourceName)
}

func CreateDelayedUpdate(clients kube.Clients, config util.Config, upgradeFuncs callbacks.RollingUpgradeFuncs, collectors metrics.Collectors, recorder record.EventRecorder, strategy invokeStrategy, item runtime.Object, delay time.Duration) error {
	resourceName := upgradeFuncs.IdFunc(item)
	logrus.Infof("Changes detected in '%s' of type '%s' in namespace '%s', and delayed reload is enabled", resourceName, upgradeFuncs.ResourceType, config.Namespace)

	logrus.Infof("Checking if delayed update for '%s' of type '%s' in namespace '%s' already exists", resourceName, upgradeFuncs.ResourceType, config.Namespace)
	if _, ok := DelayedUpdates[resourceName]; ok {
		logrus.Infof("Delayed update for '%s' of type '%s' in namespace '%s' already exists", resourceName, upgradeFuncs.ResourceType, config.Namespace)
		delayedRoll := DelayedUpdates[resourceName]
		delayedRoll.item = item
		DelayedUpdates[resourceName] = delayedRoll
		logrus.Infof("Updated '%s' of type '%s' in namespace '%s'", resourceName, upgradeFuncs.ResourceType, config.Namespace)
	} else {
		logrus.Infof("Creating delayed update for '%s' of type '%s' in namespace '%s'", resourceName, upgradeFuncs.ResourceType, config.Namespace)
		delayedRoll := DelayedUpdate{
			id:         resourceName,
			item:       item,
			rollTime:   time.Now().Add(delay),
			updateFunc: upgradeFuncs.UpdateFunc,
			namespace:  config.Namespace,
		}
		HandleDelayedUpdate(resourceName, delayedRoll, clients, upgradeFuncs, config)
	}

	return nil
}
