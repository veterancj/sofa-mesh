# Istio

[![CircleCI](https://circleci.com/gh/alipay/sofa-mesh.svg?style=shield)](https://circleci.com/gh/alipay/sofa-mesh)
[![Go Report Card](https://goreportcard.com/badge/github.com/alipay/sofa-mesh)](https://goreportcard.com/report/github.com/alipay/sofa-mesh)
[![codecov.io](https://codecov.io/github/alipay/sofa-mosn/coverage.svg?branch=master)](https://codecov.io/github/alipay/sofa-mesh?branch=master)

SOFAMesh 是基于 [Istio](https://istio.io) 改进和扩展而来的 Service Mesh 大规模落地实践方案。在继承 Istio 强大功能和丰富特性的基础上，为满足大规模部署下的性能要求以及应对落地实践中的实际情况，有如下改进：

- 采用 Golang 编写的 [MOSN](https://github.com/alipay/sofa-mosn) 取代 [Envoy](https://github.com/envoyproxy/envoy)
- 合并 Mixer 到数据平面以解决性能瓶颈
- 增强 Pilot 以实现更灵活的服务发现机制
- 增加对 [SOFA RPC](https://github.com/alipay/sofa-rpc)、Dubbo 的支持

初始版本由蚂蚁金服和阿里大文娱UC事业部携手贡献，期待社区一起来参与后续开发，共建一个开源精品项目。

- [SOFAMesh 文档](http://www.sofastack.tech/sofa-mesh/docs/Home)
- [Istio Community](https://github.com/istio/community) - describes how to get involved and contribute to the Istio project
- [Istio Developer's Guide](https://github.com/istio/istio/wiki/Dev-Guide) - explains how to set up and use an Istio development environment
- [Project Conventions](https://github.com/istio/istio/wiki/Dev-Conventions) - describes the conventions we use within the code base
- [Creating Fast and Lean Code](https://github.com/istio/istio/wiki/Dev-Writing-Fast-and-Lean-Code) - performance-oriented advice and guidelines for the code base

You'll find many other useful documents on our [Wiki](https://github.com/istio/istio/wiki).

## Introduction

Istio is an open platform for providing a uniform way to integrate
microservices, manage traffic flow across microservices, enforce policies
and aggregate telemetry data. Istio's control plane provides an abstraction
layer over the underlying cluster management platform, such as Kubernetes,
Mesos, etc.

Visit [istio.io](https://istio.io) for in-depth information about using Istio.

Istio is composed of these components:

- **Envoy** - Sidecar proxies per microservice to handle ingress/egress traffic
   between services in the cluster and from a service to external
   services. The proxies form a _secure microservice mesh_ providing a rich
   set of functions like discovery, rich layer-7 routing, circuit breakers,
   policy enforcement and telemetry recording/reporting
   functions.

  > Note: The service mesh is not an overlay network. It
  > simplifies and enhances how microservices in an application talk to each
  > other over the network provided by the underlying platform.

- **Mixer** - Central component that is leveraged by the proxies and microservices
   to enforce policies such as authorization, rate limits, quotas, authentication, request
   tracing and telemetry collection.

- **Pilot** - A component responsible for configuring the proxies at runtime.

- **Citadel** - A centralized component responsible for certificate issuance and rotation.

- **Node Agent** - A per-node component responsible for certificate issuance and rotation.

Istio currently supports Kubernetes and Consul-based environments. We plan support for additional platforms such as
Cloud Foundry, and Mesos in the near future.

## Repositories

The Istio project is divided across a few GitHub repositories.

- [istio/istio](README.md). This is the main repository that you are
currently looking at. It hosts Istio's core components and also
the sample programs and the various documents that govern the Istio open source
project. It includes:
  - [security](security/). This directory contains security related code,
including Citadel (acting as Certificate Authority), node agent, etc.
  - [pilot](pilot/). This directory
contains platform-specific code to populate the
[abstract service model](https://istio.io/docs/concepts/traffic-management/overview.html), dynamically reconfigure the proxies
when the application topology changes, as well as translate
[routing rules](https://istio.io/docs/reference/config/istio.networking.v1alpha3/) into proxy specific configuration.
  - [istioctl](istioctl/). This directory contains code for the
[_istioctl_](https://istio.io/docs/reference/commands/istioctl.html) command line utility.
  - [mixer](mixer/). This directory
contains code to enforce various policies for traffic passing through the
proxies, and collect telemetry data from proxies and services. There
are plugins for interfacing with various cloud platforms, policy
management services, and monitoring services.

- [istio/api](https://github.com/istio/api). This repository defines
component-level APIs and common configuration formats for the Istio platform.

- [istio/proxy](https://github.com/istio/proxy). The Istio proxy contains
extensions to the [Envoy proxy](https://github.com/envoyproxy/envoy) (in the form of
Envoy filters), that allow the proxy to delegate policy enforcement
decisions to Mixer.

## Issue management

We use GitHub combined with ZenHub to track all of our bugs and feature requests. Each issue we track has a variety of metadata:

- **Epic**. An epic represents a feature area for Istio as a whole. Epics are fairly broad in scope and are basically product-level things.
Each issue is ultimately part of an epic.

- **Milestone**. Each issue is assigned a milestone. This is 0.1, 0.2, ..., or 'Nebulous Future'. The milestone indicates when we
think the issue should get addressed.

- **Priority/Pipeline**. Each issue has a priority which is represented by the Pipeline field within GitHub. Priority can be one of
P0, P1, P2, or >P2. The priority indicates how important it is to address the issue within the milestone. P0 says that the
milestone cannot be considered achieved if the issue isn't resolved.

We don't annotate issues with Releases; Milestones are used instead. We don't use GitHub projects at all, that
support is disabled for our organization.
