# Product Overview

This is a Kubernetes validating admission webhook that prevents configuration conflicts between Deployments and HorizontalPodAutoscalers (HPAs).

## Core Functionality

The webhook validates two scenarios:
- **Deployment validation**: Prevents creating/updating Deployments with 1 replica when an HPA already targets them
- **HPA validation**: Prevents creating/updating HPAs that target Deployments with only 1 replica

## Purpose

HPAs require at least 2 replicas to function properly. This webhook enforces that constraint by blocking conflicting configurations that would cause HPA malfunction or resource waste.

## Target Environment

Designed for Kubernetes clusters where teams need automated prevention of HPA/Deployment misconfigurations.