module "cs-folders-iam-0-computeinstanceAdminv1" {
  source  = "terraform-google-modules/iam/google//modules/folders_iam"
  version = "~> 7.4"

  folders = [
    module.cs-envs.ids["Non-Production"],
  ]
  bindings = {
    "roles/compute.instanceAdmin.v1" = [
      "group:gcp-developers@ablegen.app",
    ]
  }
}

module "cs-folders-iam-0-containeradmin" {
  source  = "terraform-google-modules/iam/google//modules/folders_iam"
  version = "~> 7.4"

  folders = [
    module.cs-envs.ids["Non-Production"],
  ]
  bindings = {
    "roles/container.admin" = [
      "group:gcp-developers@ablegen.app",
    ]
  }
}

module "cs-folders-iam-1-computeinstanceAdminv1" {
  source  = "terraform-google-modules/iam/google//modules/folders_iam"
  version = "~> 7.4"

  folders = [
    module.cs-envs.ids["Development"],
  ]
  bindings = {
    "roles/compute.instanceAdmin.v1" = [
      "group:gcp-developers@ablegen.app",
    ]
  }
}

module "cs-folders-iam-1-containeradmin" {
  source  = "terraform-google-modules/iam/google//modules/folders_iam"
  version = "~> 7.4"

  folders = [
    module.cs-envs.ids["Development"],
  ]
  bindings = {
    "roles/container.admin" = [
      "group:gcp-developers@ablegen.app",
    ]
  }
}

module "cs-projects-iam-2-loggingviewer" {
  source  = "terraform-google-modules/iam/google//modules/projects_iam"
  version = "~> 7.4"

  projects = [
    module.cs-logging-hx964-qi555.project_id,
  ]
  bindings = {
    "roles/logging.viewer" = [
      "group:gcp-logging-viewers@ablegen.app",
    ]
  }
}

module "cs-projects-iam-2-loggingprivateLogViewer" {
  source  = "terraform-google-modules/iam/google//modules/projects_iam"
  version = "~> 7.4"

  projects = [
    module.cs-logging-hx964-qi555.project_id,
  ]
  bindings = {
    "roles/logging.privateLogViewer" = [
      "group:gcp-logging-viewers@ablegen.app",
    ]
  }
}

module "cs-projects-iam-2-bigquerydataViewer" {
  source  = "terraform-google-modules/iam/google//modules/projects_iam"
  version = "~> 7.4"

  projects = [
    module.cs-logging-hx964-qi555.project_id,
  ]
  bindings = {
    "roles/bigquery.dataViewer" = [
      "group:gcp-logging-viewers@ablegen.app",
    ]
  }
}

module "cs-projects-iam-3-bigquerydataViewer" {
  source  = "terraform-google-modules/iam/google//modules/projects_iam"
  version = "~> 7.4"

  projects = [
    module.cs-logging-hx964-qi555.project_id,
  ]
  bindings = {
    "roles/bigquery.dataViewer" = [
      "group:gcp-security-admins@ablegen.app",
    ]
  }
}

module "cs-service-projects-iam-4-computeinstanceAdminv1" {
  source  = "terraform-google-modules/iam/google//modules/projects_iam"
  version = "~> 7.4"

  projects = [
    module.cs-svc-prod1-svc-l0i8.project_id,
  ]
  bindings = {
    "roles/compute.instanceAdmin.v1" = [
      "group:${module.cs-gg-prod1-service.id}",
    ]
  }
}

module "cs-service-projects-iam-5-computeinstanceAdminv1" {
  source  = "terraform-google-modules/iam/google//modules/projects_iam"
  version = "~> 7.4"

  projects = [
    module.cs-svc-prod2-svc-l0i8.project_id,
  ]
  bindings = {
    "roles/compute.instanceAdmin.v1" = [
      "group:${module.cs-gg-prod2-service.id}",
    ]
  }
}

module "cs-service-projects-iam-6-computeinstanceAdminv1" {
  source  = "terraform-google-modules/iam/google//modules/projects_iam"
  version = "~> 7.4"

  projects = [
    module.cs-svc-nonprod1-svc-l0i8.project_id,
  ]
  bindings = {
    "roles/compute.instanceAdmin.v1" = [
      "group:${module.cs-gg-nonprod1-service.id}",
    ]
  }
}

module "cs-service-projects-iam-7-computeinstanceAdminv1" {
  source  = "terraform-google-modules/iam/google//modules/projects_iam"
  version = "~> 7.4"

  projects = [
    module.cs-svc-nonprod2-svc-l0i8.project_id,
  ]
  bindings = {
    "roles/compute.instanceAdmin.v1" = [
      "group:${module.cs-gg-nonprod2-service.id}",
    ]
  }
}
