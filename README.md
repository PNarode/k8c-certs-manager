# k8c-certs-manager
This Custom Kubernetes Controller enable developers on any Kubernetes clusters to request TLS certificates 
that they can incorporate into their application deployments.

## Description
We have a Kubernetes cluster on which we can run applications. These applications will often expose HTTP 
endpoints through which other applications and end users can access them. To make sure that a secure connection 
to those endpoints is possible, we want to provide an automated and self-service way of requesting TLS 
certificates to secure incoming traffic.Developers deploying their applications can include a custom “Certificate” 
resource with their application manifest so that they don’t need to interact with an external system to request certificates.
This Custom Controller watches a custom resource “Certificate”, processes it and creates matching TLS certificate 
secrets. Certificates issued by this controller do not need to be signed by a CA (they should be self-signed). This 
also automatically renew a Certificate when it expires.
Minimum CRD this resource required is:
```yaml
apiVersion: certs.k8c.io/v1
kind: Certificate
metadata:
  labels:
    app.kubernetes.io/name: k8c-certs-manager
    app.kubernetes.io/managed-by: kustomize
  name: certificate-sample
spec:
  dnsName: example.k8c.io
  validity: 360d
  secretRef:
    name: my-certificate-secret
```

The Event flow for the Controller is described in the diagram:

![workflow.png](workflow.png)

## Getting Started

### Prerequisites
- go version v1.21.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

## Deployment Workflow

![deployment.png](deployment.png)

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/k8s-cert-manager:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

### To Install
**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image:**

```sh
make deploy IMG=<some-registry>/k8s-cert-manager:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/certs_v1_certificate.yaml
```
This will create a Certificate resource in your Kubernetes Cluster and Create a Certificate Store at SecretRef provide as part of Resource Definition

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/certs_v1_certificate.yaml
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/k8c-certs-manager:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/k8c-certs-manager/<tag or branch>/dist/install.yaml
```

## Controller Demo

To perform the Demo make sure you have:
1. Build your DockerImage and pushed it to a public repository
2. Using DockerImage linked generate the Controller Installer. It gets generated on `dist` folder locally
3. Install and Deploy the controller:
```sh
kubectl apply -f dist/install.yaml
```
4. Make sure you have setup Ingress Controller on your Cluster. If not please follow the Steps here: https://kind.sigs.k8s.io/docs/user/ingress/
5. Once step1 to step4 are successful, you can use a Sample Application Instance to create an Application, Add Certificate and Create Ingress Controller with TLS Secrets created using your
   Controller:
```sh
kubectl apply -k config/samples/hello_app.yaml
```
```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello
spec:
  selector:
    matchLabels:
      app: hello
      version: 2.0.0
  replicas: 2
  template:
    metadata:
      labels:
        app: hello
        version: 2.0.0
    spec:
      containers:
      - name: hello
        image: "gcr.io/google-samples/hello-app:2.0"
        env:
        - name: "PORT"
          value: "50001"
---
apiVersion: v1
kind: Service
metadata:
  name: hello-service
spec:
  type: NodePort
  selector:
    app: hello
    version: 2.0.0
  ports:
  - name: http
    protocol: TCP
    port: 8081
    targetPort: 50001
---
apiVersion: certs.k8c.io/v1
kind: Certificate
metadata:
  labels:
    app.kubernetes.io/name: k8c-certs-manager
    app.kubernetes.io/managed-by: kustomize
  name: certificate-sample
spec:
  dnsName: example.k8c.io
  validity: 360d
  secretRef:
    name: my-certificate-secret
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata: 
  name: tls-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /$2
spec:
  defaultBackend:
    service:
      name: hello-service
      port:
        number: 8081	 
  ingressClassName: nginx
  tls: 
    - hosts: 
      - example.com
      secretName: my-certificate-secret
  rules: 
    - host: example.com
      http: 
        paths: 
          - path: /
            pathType: Prefix
            backend:
              service:
                name: hello-service
                port:
                  number: 8081
```
5. Once this is applied successfully add etc/hosts entry for the dns you specified pointing to `127.0.0.1`
6. Point your browser to the dnsName and you would see your application being served behind a Self Signed Certificate

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

