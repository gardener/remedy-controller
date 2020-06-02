<p>Packages:</p>
<ul>
<li>
<a href="#%22azure.remedy.gardener.cloud%22%2fv1alpha1">&#34;azure.remedy.gardener.cloud&#34;/v1alpha1</a>
</li>
</ul>
<h2 id="&#34;azure.remedy.gardener.cloud&#34;/v1alpha1">&#34;azure.remedy.gardener.cloud&#34;/v1alpha1</h2>
<p>
<p>Package v1alpha1 contains the remedy controller Azure API resources.</p>
</p>
Resource Types:
<ul><li>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.PublicIPAddress">PublicIPAddress</a>
</li></ul>
<h3 id="&#34;azure.remedy.gardener.cloud&#34;/v1alpha1.PublicIPAddress">PublicIPAddress
</h3>
<p>
<p>PublicIPAddress represents an Azure public IP address.</p>
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
&#34;azure.remedy.gardener.cloud&#34;/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>PublicIPAddress</code></td>
</tr>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.PublicIPAddressSpec">
PublicIPAddressSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.PublicIPAddressStatus">
PublicIPAddressStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="&#34;azure.remedy.gardener.cloud&#34;/v1alpha1.PublicIPAddressSpec">PublicIPAddressSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.PublicIPAddress">PublicIPAddress</a>)
</p>
<p>
</p>
<h3 id="&#34;azure.remedy.gardener.cloud&#34;/v1alpha1.PublicIPAddressStatus">PublicIPAddressStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.PublicIPAddress">PublicIPAddress</a>)
</p>
<p>
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
<code>exists</code></br>
<em>
bool
</em>
</td>
<td>
<p>Exists specifies whether the public IP address resource exists or not.</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the id of the public IP address resource in Azure.</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the public IP address resource in Azure.</p>
</td>
</tr>
<tr>
<td>
<code>ipAddress</code></br>
<em>
string
</em>
</td>
<td>
<p>IPAddres is the actual IP address of the public IP address resource in Azure.</p>
</td>
</tr>
<tr>
<td>
<code>provisioningState</code></br>
<em>
string
</em>
</td>
<td>
<p>ProvisioningState is the provisioning state of the public IP address resource in Azure.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
