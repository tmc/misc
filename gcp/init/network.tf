# VPC and Subnets
module "cs-vpc-prod-shared" {
  source  = "terraform-google-modules/network/google"
  version = "~> 5.0"

  project_id   = module.cs-vpc-host-prod-hx964-qi555.project_id
  network_name = "vpc-prod-shared"

  subnets = [
    {
      subnet_name               = "subnet-prod-1"
      subnet_ip                 = ""
      subnet_region             = ""
      subnet_private_access     = true
      subnet_flow_logs          = true
      subnet_flow_logs_sampling = "0.5"
      subnet_flow_logs_metadata = "INCLUDE_ALL_METADATA"
      subnet_flow_logs_interval = "INTERVAL_10_MIN"
    },
    {
      subnet_name               = "subnet-prod-2"
      subnet_ip                 = ""
      subnet_region             = ""
      subnet_private_access     = true
      subnet_flow_logs          = true
      subnet_flow_logs_sampling = "0.5"
      subnet_flow_logs_metadata = "INCLUDE_ALL_METADATA"
      subnet_flow_logs_interval = "INTERVAL_10_MIN"
    },
  ]

  firewall_rules = [
    {
      name      = "vpc-prod-shared-allow-icmp"
      direction = "INGRESS"
      priority  = 10000
      log_config = {
        metadata = "INCLUDE_ALL_METADATA"
      }
      allow = [
        {
          protocol = "icmp"
          ports    = []
        }
      ]
      ranges = [
        "10.128.0.0/9",
      ]
    },
    {
      name      = "vpc-prod-shared-allow-ssh"
      direction = "INGRESS"
      priority  = 10000
      log_config = {
        metadata = "INCLUDE_ALL_METADATA"
      }
      allow = [
        {
          protocol = "tcp"
          ports    = ["22"]
        }
      ]
      ranges = [
        "35.235.240.0/20",
      ]
    },
    {
      name      = "vpc-prod-shared-allow-rdp"
      direction = "INGRESS"
      priority  = 10000
      log_config = {
        metadata = "INCLUDE_ALL_METADATA"
      }
      allow = [
        {
          protocol = "tcp"
          ports    = ["3389"]
        }
      ]
      ranges = [
        "35.235.240.0/20",
      ]
    },
  ]
}

# VPC and Subnets
module "cs-vpc-nonprod-shared" {
  source  = "terraform-google-modules/network/google"
  version = "~> 5.0"

  project_id   = module.cs-vpc-host-nonprod-hx964-qi555.project_id
  network_name = "vpc-nonprod-shared"

  subnets = [
    {
      subnet_name               = "subnet-non-prod-1"
      subnet_ip                 = ""
      subnet_region             = ""
      subnet_private_access     = true
      subnet_flow_logs          = true
      subnet_flow_logs_sampling = "0.5"
      subnet_flow_logs_metadata = "INCLUDE_ALL_METADATA"
      subnet_flow_logs_interval = "INTERVAL_10_MIN"
    },
    {
      subnet_name               = "subnet-non-prod-2"
      subnet_ip                 = ""
      subnet_region             = ""
      subnet_private_access     = true
      subnet_flow_logs          = true
      subnet_flow_logs_sampling = "0.5"
      subnet_flow_logs_metadata = "INCLUDE_ALL_METADATA"
      subnet_flow_logs_interval = "INTERVAL_10_MIN"
    },
  ]

  firewall_rules = [
    {
      name      = "vpc-nonprod-shared-allow-icmp"
      direction = "INGRESS"
      priority  = 10000
      log_config = {
        metadata = "INCLUDE_ALL_METADATA"
      }
      allow = [
        {
          protocol = "icmp"
          ports    = []
        }
      ]
      ranges = [
        "10.128.0.0/9",
      ]
    },
    {
      name      = "vpc-nonprod-shared-allow-ssh"
      direction = "INGRESS"
      priority  = 10000
      log_config = {
        metadata = "INCLUDE_ALL_METADATA"
      }
      allow = [
        {
          protocol = "tcp"
          ports    = ["22"]
        }
      ]
      ranges = [
        "35.235.240.0/20",
      ]
    },
    {
      name      = "vpc-nonprod-shared-allow-rdp"
      direction = "INGRESS"
      priority  = 10000
      log_config = {
        metadata = "INCLUDE_ALL_METADATA"
      }
      allow = [
        {
          protocol = "tcp"
          ports    = ["3389"]
        }
      ]
      ranges = [
        "35.235.240.0/20",
      ]
    },
  ]
}
