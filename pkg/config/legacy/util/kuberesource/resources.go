// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kuberesource

import (
	"fmt"

	"istio.io/istio/pkg/config/legacy/processing/transformer"
	"istio.io/istio/pkg/config/schema"
	"istio.io/istio/pkg/config/schema/collection"
	"istio.io/istio/pkg/config/schema/resource"
)

// DisableExcludedCollections is a helper that filters collection.Schemas to disable some resources
// The first filter behaves in the same way as existing logic:
// - Builtin types are excluded by default.
// - If ServiceDiscovery is enabled, any built-in type should be re-added.
// In addition, any resources not needed as inputs by the specified collections are disabled
func DisableExcludedCollections(in collection.Schemas, providers transformer.Providers,
	requiredCols collection.Names, excludedResourceKinds []string, enableServiceDiscovery bool) collection.Schemas {
	// Get upstream collections in terms of transformer configuration
	// Required collections are specified in terms of transformer outputs, but we care here about the corresponding inputs
	upstreamCols := providers.RequiredInputsFor(requiredCols)

	resultBuilder := collection.NewSchemasBuilder()
	for _, s := range in.All() {
		disabled := false
		if isKindExcluded(excludedResourceKinds, s.Resource().Kind()) {
			// Found a matching exclude directive for this KubeResource. Disable the resource.
			disabled = true

			// Check and see if this is needed for Service Discovery. If needed, we will need to re-enable.
			if enableServiceDiscovery {
				if IsRequiredForServiceDiscovery(s.Resource()) {
					// This is needed for service discovery. Re-enable.
					disabled = false
				}
			}
		}

		// Additionally, filter out any resources not upstream of required collections
		if _, ok := upstreamCols[s.Name()]; !ok {
			disabled = true
		}

		if disabled {
			s = s.Disable()
		}

		_ = resultBuilder.Add(s)
	}

	return resultBuilder.Build()
}

// DefaultExcludedResourceKinds returns the default list of resource kinds to exclude.
func DefaultExcludedResourceKinds() []string {
	resources := make([]string, 0)
	for _, r := range schema.MustGet().KubeCollections().All() {
		if IsDefaultExcluded(r.Resource()) {
			resources = append(resources, r.Resource().Kind())
		}
	}
	return resources
}

func isKindExcluded(excludedResourceKinds []string, kind string) bool {
	for _, excludedKind := range excludedResourceKinds {
		if kind == excludedKind {
			return true
		}
	}

	return false
}

// the following code minimally duplicates logic from galley/pkg/config/source/kube/rt/known.go
// without propagating the many dependencies it comes with.

var knownTypes = map[string]struct{}{
	asTypesKey("", "Service"):   struct{}{},
	asTypesKey("", "Namespace"): struct{}{},
	asTypesKey("", "Node"):      struct{}{},
	asTypesKey("", "Pod"):       struct{}{},
	asTypesKey("", "Secret"):    struct{}{},
}

func asTypesKey(group, kind string) string {
	if group == "" {
		return kind
	}
	return fmt.Sprintf("%s/%s", group, kind)
}

func IsRequiredForServiceDiscovery(res resource.Schema) bool {
	key := asTypesKey(res.Group(), res.Kind())
	_, ok := knownTypes[key]
	return ok
}

func IsDefaultExcluded(res resource.Schema) bool {
	return IsRequiredForServiceDiscovery(res)
}
