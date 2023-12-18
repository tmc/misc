module "cs-vpc-host-prod-hx964-qi555" {
  source  = "terraform-google-modules/project-factory/google"
  version = "~> 14.2"

  name       = "vpc-host-prod"
  project_id = "vpc-host-prod-hx964-qi555"
  org_id     = var.org_id
  folder_id  = module.cs-common.id

  billing_account                = var.billing_account
  enable_shared_vpc_host_project = true
}

module "cs-vpc-host-nonprod-hx964-qi555" {
  source  = "terraform-google-modules/project-factory/google"
  version = "~> 14.2"

  name       = "vpc-host-nonprod"
  project_id = "vpc-host-nonprod-hx964-qi555"
  org_id     = var.org_id
  folder_id  = module.cs-common.id

  billing_account                = var.billing_account
  enable_shared_vpc_host_project = true
}

module "cs-logging-hx964-qi555" {
  source  = "terraform-google-modules/project-factory/google"
  version = "~> 14.2"

  name       = "logging"
  project_id = "logging-hx964-qi555"
  org_id     = var.org_id
  folder_id  = module.cs-common.id

  billing_account = var.billing_account
}

module "cs-monitoring-prod-hx964-qi555" {
  source  = "terraform-google-modules/project-factory/google"
  version = "~> 14.2"

  name       = "monitoring-prod"
  project_id = "monitoring-prod-hx964-qi555"
  org_id     = var.org_id
  folder_id  = module.cs-common.id

  billing_account = var.billing_account
}

module "cs-monitoring-nonprod-hx964-qi555" {
  source  = "terraform-google-modules/project-factory/google"
  version = "~> 14.2"

  name       = "monitoring-nonprod"
  project_id = "monitoring-nonprod-hx964-qi555"
  org_id     = var.org_id
  folder_id  = module.cs-common.id

  billing_account = var.billing_account
}

module "cs-monitoring-dev-hx964-qi555" {
  source  = "terraform-google-modules/project-factory/google"
  version = "~> 14.2"

  name       = "monitoring-dev"
  project_id = "monitoring-dev-hx964-qi555"
  org_id     = var.org_id
  folder_id  = module.cs-common.id

  billing_account = var.billing_account
}
