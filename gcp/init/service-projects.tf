module "cs-svc-prod1-svc-l0i8" {
  source  = "terraform-google-modules/project-factory/google//modules/svpc_service_project"
  version = "~> 14.2"

  name            = "prod1-service"
  project_id      = "prod1-svc-l0i8"
  org_id          = var.org_id
  billing_account = var.billing_account
  folder_id       = module.cs-envs.ids["Production"]

  shared_vpc = module.cs-vpc-host-prod-hx964-qi555.project_id

  domain     = data.google_organization.org.domain
  group_name = module.cs-gg-prod1-service.name
  group_role = "roles/viewer"
}

module "cs-svc-prod2-svc-l0i8" {
  source  = "terraform-google-modules/project-factory/google//modules/svpc_service_project"
  version = "~> 14.2"

  name            = "prod2-service"
  project_id      = "prod2-svc-l0i8"
  org_id          = var.org_id
  billing_account = var.billing_account
  folder_id       = module.cs-envs.ids["Production"]

  shared_vpc = module.cs-vpc-host-prod-hx964-qi555.project_id

  domain     = data.google_organization.org.domain
  group_name = module.cs-gg-prod2-service.name
  group_role = "roles/viewer"
}

module "cs-svc-nonprod1-svc-l0i8" {
  source  = "terraform-google-modules/project-factory/google//modules/svpc_service_project"
  version = "~> 14.2"

  name            = "nonprod1-service"
  project_id      = "nonprod1-svc-l0i8"
  org_id          = var.org_id
  billing_account = var.billing_account
  folder_id       = module.cs-envs.ids["Non-Production"]

  shared_vpc = module.cs-vpc-host-nonprod-hx964-qi555.project_id

  domain     = data.google_organization.org.domain
  group_name = module.cs-gg-nonprod1-service.name
  group_role = "roles/viewer"
}

module "cs-svc-nonprod2-svc-l0i8" {
  source  = "terraform-google-modules/project-factory/google//modules/svpc_service_project"
  version = "~> 14.2"

  name            = "nonprod2-service"
  project_id      = "nonprod2-svc-l0i8"
  org_id          = var.org_id
  billing_account = var.billing_account
  folder_id       = module.cs-envs.ids["Non-Production"]

  shared_vpc = module.cs-vpc-host-nonprod-hx964-qi555.project_id

  domain     = data.google_organization.org.domain
  group_name = module.cs-gg-nonprod2-service.name
  group_role = "roles/viewer"
}
