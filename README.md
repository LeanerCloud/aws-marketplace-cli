# aws-marketplace-cli

A friendlier way to manage your AWS Marketplace products from the command line and based on the configuration persisted in source control

## Why?

- Because the AWS Marketplace is a PITA to manage from the GUI
- Because doing it from the AWS CLI is even less user friendy

## Installation Instructions

You need to a working Go installation, you can get Go from [https://go.dev/](https://go.dev/), then run the following command:

```bash
go install github.com/LeanerCloud/aws-marketplace-cli
```

## Usage

- Authenticate to your AWS account using credentials using the usual environment variables.

- List all your AWS Marketplace products:

```bash
$ aws-marketplace-cli list ContainerProduct
AutoSpotting
EBS Optimizer
```

- Dump a given product to a YAML file in the current working directory:

```bash
$ aws-marketplace-cli dump AutoSpotting
Data written to products/AutoSpotting/description.yaml
````

- Have a look at the YAML configuration, and feel free to edit it at will with your favorite editor.

``` bash
$ cat products/AutoSpotting/description.yaml
description:
  highlights:
  - Up to 90% cost savings by automatically replacing On-Demand AutoScaling group
    nodes with identically configured Spot instances
  - Increased instance type diversification and failover to on-demand for high availability.
  - Can keep a percentage or number of On-Demand capacity running in your AutoScaling
    groups.
  longdescription: |-
    All you need to do is:

    1. Install AutoSpotting from the AWS Marketplace by using CloudFormation or
    Terraform. You need to click `Continue to Subscribe` on the top right and follow the
    instructions until the end.

```

- Once you have edited the YAML configuration, you can apply it to your AWS Marketplace product:

```bash
$ aws-marketplace-cli update-description AutoSpotting
Updating product
Changeset created successfully
```

- Feel free to persist this YAML file in your source control, and maybe maintain it as a private fork.


## Current Status

- supports only updating the basic product information.


## Potential future work (contributions welcome!)

- Add support for managing new versions of the product and potentially other changes.
- Add support for managing AMI products

## License

This software is licensed under the GNU General Public License V3
