## MODIFIED Requirements

### Requirement: Resolver contract

The system SHALL define a resolver contract that is not tied to any single
domain. A resolver MUST declare, via a **matcher**, which references it handles.
Given a reference it handles, a resolver MUST retrieve the underlying object
**itself** (through shared clients provided in a resolve context) and return that
object's **health** plus a list of **child references** to traverse next. A
resolver MUST also declare, for a reference it handles, whether that reference is
**expandable** — that is, whether it can descend to a child layer — without
retrieving the object. Resolvers MUST be read-only. Child references MAY belong
to a **different domain** than the resolver that produced them.

#### Scenario: Resolver retrieves and reports children and health

- **WHEN** the engine invokes a resolver with a reference it matches
- **THEN** the resolver retrieves the object via the resolve context and returns a health status and a (possibly empty) list of child references

#### Scenario: Resolver claims references via its matcher

- **WHEN** the registry tests a reference against a resolver's matcher
- **THEN** the matcher decides, from the reference's domain and type, whether that resolver handles it

#### Scenario: Resolver declares expandability

- **WHEN** a resolver is asked whether a reference it matches is expandable
- **THEN** it answers from the reference alone, without retrieving the object and without erroring

#### Scenario: Children may switch domains

- **WHEN** a resolver in domain `kubernetes` determines its object maps to an object in domain `gcp`
- **THEN** it returns child references in domain `gcp`, and the engine traverses them with a `gcp` resolver without any special-casing in the engine

