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
</tbody>
</table>
<hr/>
