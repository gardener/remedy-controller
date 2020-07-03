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
</li><li>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.VirtualMachine">VirtualMachine</a>
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
<h3 id="&#34;azure.remedy.gardener.cloud&#34;/v1alpha1.VirtualMachine">VirtualMachine
</h3>
<p>
<p>VirtualMachine represents an Azure virtual machine.</p>
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
<td><code>VirtualMachine</code></td>
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
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.VirtualMachineSpec">
VirtualMachineSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>hostname</code></br>
<em>
string
</em>
</td>
<td>
<p>Hostname is the hostname of the Kubernetes node for this virtual machine.</p>
</td>
</tr>
<tr>
<td>
<code>providerID</code></br>
<em>
string
</em>
</td>
<td>
<p>ProviderID is the provider ID of the Kubernetes node for this virtual machine.</p>
</td>
</tr>
<tr>
<td>
<code>notReadyOrUnreachable</code></br>
<em>
bool
</em>
</td>
<td>
<p>NotReadyOrUnreachable is whether the Kubernetes node for this virtual machine is either not ready or unreachable.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.VirtualMachineStatus">
VirtualMachineStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="&#34;azure.remedy.gardener.cloud&#34;/v1alpha1.FailedOperation">FailedOperation
</h3>
<p>
(<em>Appears on:</em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.PublicIPAddressStatus">PublicIPAddressStatus</a>, 
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.VirtualMachineStatus">VirtualMachineStatus</a>)
</p>
<p>
<p>FailedOperation describes a failed Azure operation that has been attempted a certain number of times.</p>
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
<code>type</code></br>
<em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.OperationType">
OperationType
</a>
</em>
</td>
<td>
<p>Type is the operation type.</p>
</td>
</tr>
<tr>
<td>
<code>attempts</code></br>
<em>
int
</em>
</td>
<td>
<p>Attempts is the number of times the operation was attempted so far.</p>
</td>
</tr>
<tr>
<td>
<code>errorMessage</code></br>
<em>
string
</em>
</td>
<td>
<p>ErrorMessage is a the error message from the last operation failure.</p>
</td>
</tr>
<tr>
<td>
<code>timestamp</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>Timestamp is the timestamp of the last operation failure.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="&#34;azure.remedy.gardener.cloud&#34;/v1alpha1.OperationType">OperationType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.FailedOperation">FailedOperation</a>)
</p>
<p>
<p>OperationType is a string alias.</p>
</p>
<h3 id="&#34;azure.remedy.gardener.cloud&#34;/v1alpha1.PublicIPAddressSpec">PublicIPAddressSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.PublicIPAddress">PublicIPAddress</a>)
</p>
<p>
<p>PublicIPAddressSpec represents the spec of an Azure public IP address.</p>
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
<code>ipAddress</code></br>
<em>
string
</em>
</td>
<td>
<p>IPAddres is the actual IP address of the public IP address resource in Azure.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="&#34;azure.remedy.gardener.cloud&#34;/v1alpha1.PublicIPAddressStatus">PublicIPAddressStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.PublicIPAddress">PublicIPAddress</a>)
</p>
<p>
<p>PublicIPAddressStatus represents the status of an Azure public IP address.</p>
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
<code>provisioningState</code></br>
<em>
string
</em>
</td>
<td>
<p>ProvisioningState is the provisioning state of the public IP address resource in Azure.</p>
</td>
</tr>
<tr>
<td>
<code>failedOperations</code></br>
<em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.FailedOperation">
[]FailedOperation
</a>
</em>
</td>
<td>
<p>FailedOperations is a list of all failed operations on the virtual machine resource in Azure.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="&#34;azure.remedy.gardener.cloud&#34;/v1alpha1.VirtualMachineSpec">VirtualMachineSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.VirtualMachine">VirtualMachine</a>)
</p>
<p>
<p>VirtualMachineSpec represents the spec of an Azure virtual machine.</p>
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
<code>hostname</code></br>
<em>
string
</em>
</td>
<td>
<p>Hostname is the hostname of the Kubernetes node for this virtual machine.</p>
</td>
</tr>
<tr>
<td>
<code>providerID</code></br>
<em>
string
</em>
</td>
<td>
<p>ProviderID is the provider ID of the Kubernetes node for this virtual machine.</p>
</td>
</tr>
<tr>
<td>
<code>notReadyOrUnreachable</code></br>
<em>
bool
</em>
</td>
<td>
<p>NotReadyOrUnreachable is whether the Kubernetes node for this virtual machine is either not ready or unreachable.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="&#34;azure.remedy.gardener.cloud&#34;/v1alpha1.VirtualMachineStatus">VirtualMachineStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.VirtualMachine">VirtualMachine</a>)
</p>
<p>
<p>VirtualMachineStatus represents the status of an Azure virtual machine.</p>
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
<p>Exists specifies whether the virtual machine resource exists or not.</p>
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
<p>ID is the id of the virtual machine resource in Azure.</p>
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
<p>Name is the name of the virtual machine resource in Azure.</p>
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
<p>ProvisioningState is the provisioning state of the virtual machine resource in Azure.</p>
</td>
</tr>
<tr>
<td>
<code>failedOperations</code></br>
<em>
<a href="#%22azure.remedy.gardener.cloud%22/v1alpha1.FailedOperation">
[]FailedOperation
</a>
</em>
</td>
<td>
<p>FailedOperations is a list of all failed operations on the virtual machine resource in Azure.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
