// Copyright 2021-2022 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"

	"google.golang.org/protobuf/types/known/wrapperspb"

	policyv1 "github.com/cerbos/cerbos/api/genpb/cerbos/policy/v1"
	"github.com/cerbos/cerbos/internal/namer"
	"github.com/cerbos/cerbos/internal/util"
)

// Kind defines the type of policy (resource, principal, derived_roles etc.).
type Kind int

const (
	// ResourceKind points to a resource policy.
	ResourceKind Kind = iota
	PrincipalKind
	DerivedRolesKind
)

const (
	ResourceKindStr     = "RESOURCE"
	PrincipalKindStr    = "PRINCIPAL"
	DerivedRolesKindStr = "DERIVED_ROLES"
)

var IgnoreHashFields = map[string]struct{}{
	"cerbos.policy.v1.Policy.metadata":    {},
	"cerbos.policy.v1.Policy.disabled":    {},
	"cerbos.policy.v1.Policy.description": {},
}

func (k Kind) String() string {
	switch k {
	case ResourceKind:
		return ResourceKindStr
	case PrincipalKind:
		return PrincipalKindStr
	case DerivedRolesKind:
		return DerivedRolesKindStr
	default:
		panic(fmt.Errorf("unknown policy kind %d", k))
	}
}

// GetKind returns the kind of the given policy.
func GetKind(p *policyv1.Policy) Kind {
	switch pt := p.PolicyType.(type) {
	case *policyv1.Policy_ResourcePolicy:
		return ResourceKind
	case *policyv1.Policy_PrincipalPolicy:
		return PrincipalKind
	case *policyv1.Policy_DerivedRoles:
		return DerivedRolesKind
	default:
		panic(fmt.Errorf("unknown policy type %T", pt))
	}
}

// Dependencies returns the module names of dependencies of the policy.
func Dependencies(p *policyv1.Policy) []string {
	switch pt := p.PolicyType.(type) {
	case *policyv1.Policy_ResourcePolicy:
		imports := pt.ResourcePolicy.ImportDerivedRoles
		if len(imports) == 0 {
			return nil
		}

		dr := make([]string, len(imports))
		for i, imp := range imports {
			dr[i] = namer.DerivedRolesFQN(imp)
		}

		return dr
	default:
		return nil
	}
}

// SchemaReferences returns references to the schemas found in the policy.
func SchemaReferences(p *policyv1.Policy) []string {
	switch pt := p.PolicyType.(type) {
	case *policyv1.Policy_ResourcePolicy:
		schemas := pt.ResourcePolicy.Schemas
		if schemas == nil {
			return nil
		}

		var refs []string
		if schemas.PrincipalSchema != nil && schemas.PrincipalSchema.Ref != "" {
			refs = append(refs, schemas.PrincipalSchema.Ref)
		}

		if schemas.ResourceSchema != nil && schemas.ResourceSchema.Ref != "" {
			refs = append(refs, schemas.ResourceSchema.Ref)
		}

		return refs
	default:
		return nil
	}
}

// WithMetadata adds metadata to the policy.
func WithMetadata(p *policyv1.Policy, source string, annotations map[string]string, storeIdentifier string) *policyv1.Policy {
	if p.Metadata == nil {
		p.Metadata = &policyv1.Metadata{}
	}

	p.Metadata.SourceFile = source
	p.Metadata.Annotations = annotations

	if p.Metadata.StoreIdentifer == "" {
		p = WithStoreIdentifier(p, storeIdentifier)
	}

	if p.Metadata.Hash == nil {
		return WithHash(p)
	}

	return p
}

// WithStoreIdentifier adds the store identifier to the metadata.
func WithStoreIdentifier(p *policyv1.Policy, storeIdentifier string) *policyv1.Policy {
	if p.Metadata == nil {
		p.Metadata = &policyv1.Metadata{}
	}

	p.Metadata.StoreIdentifer = storeIdentifier

	return p
}

// WithHash calculates the hash for the policy and adds it to metadata.
func WithHash(p *policyv1.Policy) *policyv1.Policy {
	if p.Metadata == nil {
		p.Metadata = &policyv1.Metadata{}
	}

	p.Metadata.Hash = wrapperspb.UInt64(util.HashPB(p, IgnoreHashFields))

	return p
}

// GetHash returns the hash of the policy.
func GetHash(p *policyv1.Policy) uint64 {
	if p.Metadata == nil || p.Metadata.Hash == nil {
		p = WithHash(p)
	}

	return p.Metadata.Hash.GetValue()
}

// GetSourceFile gets the source file name from metadata if it exists.
func GetSourceFile(p *policyv1.Policy) string {
	if p == nil {
		return "unknown<nil>"
	}

	if p.Metadata != nil && p.Metadata.SourceFile != "" {
		return p.Metadata.SourceFile
	}

	return fmt.Sprintf("unknown<%s>", namer.FQN(p))
}

// Wrapper is a convenience layer over the policy definition.
type Wrapper struct {
	*policyv1.Policy
	FQN          string
	Kind         string
	Name         string
	Version      string
	Dependencies []namer.ModuleID
	ID           namer.ModuleID
}

// Wrap augments a policy with useful information about itself.
func Wrap(p *policyv1.Policy) Wrapper {
	w := Wrapper{Policy: p}

	switch pt := p.PolicyType.(type) {
	case *policyv1.Policy_ResourcePolicy:
		w.Kind = ResourceKind.String()
		w.FQN = namer.ResourcePolicyFQN(pt.ResourcePolicy.Resource, pt.ResourcePolicy.Version)
		w.ID = namer.GenModuleIDFromFQN(w.FQN)
		w.Name = pt.ResourcePolicy.Resource
		w.Version = pt.ResourcePolicy.Version

		imports := pt.ResourcePolicy.ImportDerivedRoles
		if len(imports) > 0 {
			w.Dependencies = make([]namer.ModuleID, len(imports))
			for i, imp := range imports {
				w.Dependencies[i] = namer.GenModuleIDFromFQN(namer.DerivedRolesFQN(imp))
			}
		}

	case *policyv1.Policy_PrincipalPolicy:
		w.Kind = PrincipalKind.String()
		w.FQN = namer.PrincipalPolicyFQN(pt.PrincipalPolicy.Principal, pt.PrincipalPolicy.Version)
		w.ID = namer.GenModuleIDFromFQN(w.FQN)
		w.Name = pt.PrincipalPolicy.Principal
		w.Version = pt.PrincipalPolicy.Version

	case *policyv1.Policy_DerivedRoles:
		w.Kind = DerivedRolesKind.String()
		w.FQN = namer.DerivedRolesFQN(pt.DerivedRoles.Name)
		w.ID = namer.GenModuleIDFromFQN(w.FQN)
		w.Name = pt.DerivedRoles.Name

	default:
		panic(fmt.Errorf("unknown policy type %T", pt))
	}

	return w
}

// CompilationUnit is the set of policies that need to be compiled together.
// For example, if a resource policy named R imports derived roles named D, the compilation unit will contain
// both R and D with the ModID field pointing to R because it is the main policy.
type CompilationUnit struct {
	Definitions map[namer.ModuleID]*policyv1.Policy
	ModID       namer.ModuleID
}

func (cu *CompilationUnit) AddDefinition(id namer.ModuleID, p *policyv1.Policy) {
	if cu.Definitions == nil {
		cu.Definitions = make(map[namer.ModuleID]*policyv1.Policy)
	}

	cu.Definitions[id] = p
}

func (cu *CompilationUnit) MainSourceFile() string {
	return GetSourceFile(cu.Definitions[cu.ModID])
}

func (cu *CompilationUnit) MainPolicy() *policyv1.Policy {
	return cu.Definitions[cu.ModID]
}

// Key returns the human readable identifier for the main module.
func (cu *CompilationUnit) Key() string {
	return namer.PolicyKey(cu.Definitions[cu.ModID])
}
