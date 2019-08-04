package services

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/models"
	apps_v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type PortMappingChecker struct {
	Service     v1.Service
	Deployments []apps_v1.Deployment
}

func (p PortMappingChecker) Check() ([]*models.IstioCheck, bool) {
	validations := make([]*models.IstioCheck, 0)

	for portIndex, sp := range p.Service.Spec.Ports {
		if strings.ToLower(string(sp.Protocol)) == "udp" {
			continue
		} else if !kubernetes.MatchPortNameWithValidProtocols(sp.Name) {
			validation := models.Build("port.name.mismatch", fmt.Sprintf("spec/ports[%d]", portIndex))
			validations = append(validations, &validation)
		}
	}
	if deployment := p.findMatchingDeployment(p.Service.Spec.Selector); deployment != nil {
		p.matchPorts(&p.Service, deployment, &validations)
	}

	return validations, len(validations) == 0
}

func (p PortMappingChecker) findMatchingDeployment(selectors map[string]string) *apps_v1.Deployment {
	if len(selectors) == 0 {
		return nil
	}

	selector := labels.SelectorFromSet(labels.Set(selectors))

	for _, d := range p.Deployments {
		labelSet := labels.Set(d.Labels)

		if selector.Matches(labelSet) {
			return &d
		}
	}
	return nil
}

func (p PortMappingChecker) matchPorts(service *v1.Service, deployment *apps_v1.Deployment, validations *[]*models.IstioCheck) {
Service:
	for portIndex, sp := range service.Spec.Ports {
		if sp.TargetPort.Type == intstr.String && sp.TargetPort.StrVal != "" {
			// Check port name in this case
			for _, c := range deployment.Spec.Template.Spec.Containers {
				for _, cp := range c.Ports {
					if cp.Name == sp.TargetPort.StrVal {
						continue Service
					}
				}
			}
		} else {
			portNumber := sp.Port
			if sp.TargetPort.Type == intstr.Int && sp.TargetPort.IntVal > 0 {
				// Check port number from here
				portNumber = sp.TargetPort.IntVal
			}
			for _, c := range deployment.Spec.Template.Spec.Containers {
				for _, cp := range c.Ports {
					if cp.ContainerPort == portNumber {
						continue Service
					}
				}
			}
		}
		validation := models.Build("service.deployment.port.mismatch", fmt.Sprintf("spec/ports[%d]", portIndex))
		*validations = append(*validations, &validation)
	}
}
