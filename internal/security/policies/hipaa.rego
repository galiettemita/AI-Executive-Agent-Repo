package brevio.hipaa

import future.keywords.if
import future.keywords.in

default allow_health_domain = false

# Workspace must be hipaa_covered_entity
has_hipaa_workspace if {
    input.workspace_type == "hipaa_covered_entity"
}

# Active BAA must exist
has_active_baa if {
    some baa in data.business_associate_agreements
    baa.workspace_id == input.workspace_id
    baa.revoked_at == null
}

# HIPAA_AUTH consent must be granted
has_hipaa_consent if {
    some consent in data.active_consents
    consent.workspace_id == input.workspace_id
    consent.user_id == input.user_id
    consent.purpose == "executive_assistance"
    consent.revoked_at == null
}

allow_health_domain if {
    has_hipaa_workspace
    has_active_baa
    has_hipaa_consent
}

deny_reason := "workspace is not a HIPAA covered entity" if {
    not has_hipaa_workspace
}

deny_reason := "BAA required" if {
    has_hipaa_workspace
    not has_active_baa
}

deny_reason := "HIPAA_AUTH consent required" if {
    has_hipaa_workspace
    has_active_baa
    not has_hipaa_consent
}
