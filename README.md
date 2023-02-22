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
Product: AutoSpotting    EntityID: 9ea9ac37-bdfe-49aa-a756-e9fde98cc210
Product: EBS Optimizer   EntityID: a8ee4609-273e-4666-8dcc-fc101bff1618
```

- Dump a given product to a YAML file in the current working directory:

```bash
$ aws-marketplace-cli dump 9ea9ac37-bdfe-49aa-a756-e9fde98cc210
Data written to AutoSpotting_9ea9ac37-bdfe-49aa-a756-e9fde98cc210.yaml
````

- Have a look at the YAML configuration, and feel free to edit it at will with your favorite editor.

``` bash
$ cat AutoSpotting_9ea9ac37-bdfe-49aa-a756-e9fde98cc210.yaml
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

$ aws-marketplace-cli update-description AutoSpotting_9ea9ac37-bdfe-49aa-a756-e9fde98cc210.yaml
Updating product
Changeset created successfully

- Feel free to persist this YAML file in your source control, and maybe maintain it as a private fork.


## Current Status

- supports only updating the basic product information.


## Potential future work (contributions welcome!)

- Add support for managing new versions of the product and potentially other changes.
- Maybe reorganize the product YAML data into a subdirectory.
- Add support for managing AMI products
- Prettier display of the product listing

## License

This software is licensed under the GNU General Public License V3
