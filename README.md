# ot-sync-operator
K8s operator designed to orchestrate the syncing of datavolumes required for workspace instances launched in OpenTerrian

## Description
This project is currently in active development and should be considered pre-alpha in its current state.

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- kind version v0.29.0+
- Access to a Kubernetes v1.11.3+ cluster.

### Local Development

To develop locally the following steps must be taken to run the application.


#### Init the cluster

To init the cluster please run

```bash
make setup-test-e2e
```

This command will stand up a kind cluster locally. The cluster will have all all dependencies installed.

> **NOTE**: Please ensure you are using the correct context before running the below commands. We are installing stuff into the cluster.

Next run the below command to install our CRD onto your cluster.

```bash
make install
```

Finally please install the dependencies required for the operator to run. Currently the operator requires a secret and configmap to be installed in the same namespace as it. These resources are required for pulling from s3 and other resources. In the actual deployed operator we will check for these in a initContainer however we still look for them in the code as well.

To install please run the below command.

> **NOTE**: The yamls in the test/secret-yamls directory contain only dummy values. You will need to populate them with real values if you want to pull locally from a registry. Instructions on what to do to get the correct values can be found in the yaml files within the directory.


```bash
kubectl apply -f test/secret-yamls
```

#### Developing locally

To start the operator outside of the cluster please run the below command. This will run the operator on your machine. Please not that this does not support hot reloading. You will need to restart the server as they make changes.

```bash
make run
```

##### Making Changes to the shape of our CRD.

If you need to make changes to the shape of the CRD (ie. changing the datasync_types file) you will need to regenerate manifests and code created via kubebuilder. To do so please run the following commands in order.

The below command will uninstall your CRD from the cluster.
```bash
make uninstall
```

Then run these commands to regenerate the code and manifests.

```bash
make generate
make manifests
```

Finally re-install your CRD.
```
make install
```

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/ot-sync-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/ot-sync-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
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

Following the options to release and provide this solution to the users.

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.



