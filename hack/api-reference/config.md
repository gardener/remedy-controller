<p>Packages:</p>
<ul>
<li>
<a href="#%22remedy.config.gardener.cloud%22%2fv1alpha1">&#34;remedy.config.gardener.cloud&#34;/v1alpha1</a>
</li>
</ul>
<h2 id="&#34;remedy.config.gardener.cloud&#34;/v1alpha1">&#34;remedy.config.gardener.cloud&#34;/v1alpha1</h2>
<p>
<p>Package v1alpha1 contains the remedy controller configuration API resources.</p>
</p>
Resource Types:
<ul><li>
<a href="#%22remedy.config.gardener.cloud%22/v1alpha1.ControllerConfiguration">ControllerConfiguration</a>
</li></ul>
<h3 id="&#34;remedy.config.gardener.cloud&#34;/v1alpha1.ControllerConfiguration">ControllerConfiguration
</h3>
<p>
<p>ControllerConfiguration defines the configuration for the GCP provider.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
&#34;remedy.config.gardener.cloud&#34;/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>ControllerConfiguration</code></td>
</tr>
<tr>
<td>
<code>clientConnection</code></br>
<em>
<a href="https://godoc.org/k8s.io/component-base/config/v1alpha1#ClientConnectionConfiguration">
Kubernetes v1alpha1.ClientConnectionConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClientConnection specifies the kubeconfig file and client connection
settings for the proxy server to use when communicating with the apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>azure</code></br>
<em>
<a href="#%22remedy.config.gardener.cloud%22/v1alpha1.AzureConfiguration">
AzureConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Azure specifies the configuration for all Azure remedies.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="&#34;remedy.config.gardener.cloud&#34;/v1alpha1.AzureConfiguration">AzureConfiguration
</h3>
<p>
(<em>Appears on:</em>
<a href="#%22remedy.config.gardener.cloud%22/v1alpha1.ControllerConfiguration">ControllerConfiguration</a>)
</p>
<p>
<p>AzureConfiguration defines the configuration for the Azure remedy controller.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>publicIPRemedy</code></br>
<em>
<a href="#%22remedy.config.gardener.cloud%22/v1alpha1.AzurePublicIPRemedyConfiguration">
AzurePublicIPRemedyConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="&#34;remedy.config.gardener.cloud&#34;/v1alpha1.AzurePublicIPRemedyConfiguration">AzurePublicIPRemedyConfiguration
</h3>
<p>
(<em>Appears on:</em>
<a href="#%22remedy.config.gardener.cloud%22/v1alpha1.AzureConfiguration">AzureConfiguration</a>)
</p>
<p>
<p>AzurePublicIPRemedyConfiguration defines the configuration for the public IP remedy.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>requeueInterval</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#duration-v1-meta">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RequeueInterval specifies the time after which reconciliation requests will be
requeued. Applies to both creation/update and deletion.</p>
</td>
</tr>
<tr>
<td>
<code>deletionGracePeriod</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#duration-v1-meta">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DeletionGracePeriod specifies the period after which a public ip address will be
deleted by the controller if it still exists.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
