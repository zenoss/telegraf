package zenoss


var vspherConfig =`
###############################################################################
#                            ZENOSS PROCESSOR PLUGINS                         #
###############################################################################

#########################################
## Zenoss specific processor for vsphere input
#########################################
[[processors.rename]]
  order=0
  namepass=["vsphere*"]
  tagexclude=['host']
  ## Specify one sub-table per rename operation.
  [[processors.rename.replace]]
    tag = "source"
    dest = "entity_source"


#########################################
# Zenoss  processor for vsphere cluster
#########################################
[[processors.rename]]
  order=0
  namepass=["vsphere_cluster_clusterServices", "vsphere_cluster_vmop"]
  ## Specify one sub-table per rename operation.
  [[processors.rename.replace]]
    tag = "vcenter"
    dest = "zdim_vcenter"
  [[processors.rename.replace]]
    tag = "dcname"
    dest = "zdim_dcname"
  [[processors.rename.replace]]
    tag = "clustername"
    dest = "zdim_clustername"  

[[processors.template]]
  order=1
  namepass=["vsphere_cluster_clusterServices", "vsphere_cluster_vmop"]
  tag = "type"
  template = 'vsphere.cluster'

[[processors.template]]
  order=1
  namepass=["vsphere_cluster_clusterServices", "vsphere_cluster_vmop"]
  tag = "zname"
  template = '{{ .Tag "zdim_vcenter" }}/{{ .Tag "zdim_dcname" }}/{{ .Tag "zdim_clustername"}}'


#########################################
# Zenoss  processor for vsphere datastore
# TODO: these probably won't cut it on a 
# vcenter with multiple datastores
#########################################
[[processors.rename]]
  order=0
  namepass=["vsphere_datastore_datastore"]
  ## Specify one sub-table per rename operation.
  [[processors.rename.replace]]
    tag = "vcenter"
    dest = "zdim_vcenter"
  [[processors.rename.replace]]
    tag = "dcname"
    dest = "zdim_dcname"
[[processors.template]]
  order=1
  namepass=["vsphere_datastore_datastore"]
  tag = "zdim_datastore"
  template = '{{ .Tag "entity_source" }}'    
[[processors.template]]
  order=2
  namepass=["vsphere_datastore_datastore"]
  tag = "zname"
  template = '{{ .Tag "zdim_vcenter" }}/{{ .Tag "zdim_dcname" }}/{{ .Tag "zdim_datastore"}}'

[[processors.template]]
  order=1
  namepass=["vsphere_datastore_datastore"]
  tag = "type"
  template = 'vsphere.datastore'

#########################################
# Zenoss  processor for vsphere datacenter
#########################################
[[processors.rename]]
  order=0
  namepass=["vsphere_datacenter_vmop"]
  ## Specify one sub-table per rename operation.
  [[processors.rename.replace]]
    tag = "vcenter"
    dest = "zdim_vcenter"
  [[processors.rename.replace]]
    tag = "dcname"
    dest = "zdim_dcname"
[[processors.template]]
  order=1
  namepass=["vsphere_datacenter_vmop"]
  tag = "zname"
  template = '{{ .Tag "zdim_vcenter" }}/{{ .Tag "zdim_dcname" }}'

[[processors.template]]
  order=1
  namepass=["vsphere_datacenter_vmop"]
  tag = "type"
  template = 'vsphere.datacenter'

#########################################
# Zenoss shared processor for vsphere hosts
#########################################
[[processors.template]]
  order=0
  namepass=["vsphere_host_mem", "vsphere_host_sys", "vsphere_host_storageAdapter", "vsphere_host_power", "vsphere_host_cpu", "vsphere_host_net", "vsphere_host_disk"]
  tagexclude=['host']
  tag = "zdim_host"
  template = '{{ .Tag "esxhostname" }}'
[[processors.rename]]
  order=0
  namepass=["vsphere_host_mem", "vsphere_host_sys", "vsphere_host_storageAdapter", "vsphere_host_power", "vsphere_host_cpu", "vsphere_host_net", "vsphere_host_disk"]
  ## Specify one sub-table per rename operation.
  [[processors.rename.replace]]
    tag = "vcenter"
    dest = "zdim_vcenter"
  [[processors.rename.replace]]
    tag = "dcname"
    dest = "zdim_dcname"

# # impact to host from cpu, disk and net
[[processors.template]]
  order=200
  namepass=["vsphere_host_mem", "vsphere_host_sys", "vsphere_host_storageAdapter", "vsphere_host_power", "vsphere_host_cpu", "vsphere_host_net", "vsphere_host_disk"]
  tag = "impactToDimensions"
  # template = 'vcenter={{ .Tag "zdim_vcenter" }},dcname={{ .Tag "zdim_dcname" }},host={{ .Tag "esxhostname" }}'
  template = '''source=zenoss.telegraf.truffle.local,vcenter={{- .Tag "zdim_vcenter" -}},dcname={{- .Tag "zdim_dcname" -}}
                {{if not (eq (.Tag "type") "vsphere.host") }},host={{ .Tag "zdim_host" }}{{- end -}}
              '''

#host type processors
[[processors.template]]
  # determine types for host and subtypes
  #needs to be last processor for vsphere_host metrics
  order=100
  namepass=["vsphere_host_mem", "vsphere_host_sys", "vsphere_host_power","vsphere_host_cpu", "vsphere_host_disk", "vsphere_host_net", "vsphere_host_storageAdapter"]
  tag = "type"
  template = 'vsphere.{{if .Tag "zdim_cpu" }}cpu{{else if .Tag "zdim_interface" }}interface{{else if .Tag "zdim_disk" }}disk{{else if .Tag "zdim_adapter" }}storageAdapter{{else}}host{{end}}'

###################################
# name for top level host metrics
###################################
## Zenoss processor for vsphere_vm_mem vsphere_vm_sys vsphere_vm_power
# Name for hosts s
[[processors.template]]
  order=1
  namepass=["vsphere_host_mem", "vsphere_host_sys", "vsphere_host_power"]
  tag = "zname"
  template = '{{ .Tag "zdim_vcenter" }}/{{ .Tag "zdim_dcname" }}/{{ .Tag "zdim_host" }}'

###################################
# vsphere_host_cpu
###################################
## determine cpu dimension - blank any cpu name for instance-total 
## instance-total metrics belong to the host not the cpu entities
[[processors.regex]]
  order=1
  namepass = ["vsphere_host_cpu"]
   # Tag and field conversions defined in a separate sub-tables
  [[processors.regex.tags]]
    ## Tag to change
    key = "cpu"
    ## Regular expression to match on a tag value
    # match disk tags that have numeric values
    pattern = "instance-total"
    ## Matches of the pattern will be replaced with this string.  Use ${1}
    ## notation to use the text of the first submatch.
    #set to white space, empty string don't work
    replacement = " "

## Copy any non-blank disk tags to zdim_disk - order is important relative to previous regex
[[processors.regex]]
  order=2
  namepass = ["vsphere_host_cpu"]
   # Tag and field conversions defined in a separate sub-tables
  [[processors.regex.tags]]
    ## Tag to change
    key = "cpu"
    ## Regular expression to match on a tag value
    # match cpu tags that have numeric values
    pattern = "(\\S+)"
    ## Matches of the pattern will be replaced with this string.  Use ${1}
    ## notation to use the text of the first submatch.
    #keep the whole value
    replacement = "${1}"
    ## If result_key is present, a new field will be created
    ## instead of changing existing field
    # set the value on a new tag
    result_key = "zdim_cpu"

# Name for host cpu
[[processors.template]]
  order=3
  namepass=["vsphere_host_cpu"]
  #ignore cpu tag as zdim_cpu is what we want
  tagexclude=["cpu"]
  tag = "zname"
  template = '{{ .Tag "zdim_vcenter" }}/{{ .Tag "zdim_dcname" }}/{{ .Tag "zdim_host" }}{{if .Tag "zdim_cpu" }}/{{ .Tag "zdim_cpu" }}{{end}}'

###################################
# vsphere_host_disk
###################################
## determine disk dimension - blank any disk name for instance-total 
## instance-total metrics belong to the host not the disk entities
[[processors.regex]]
  order=1
  namepass = ["vsphere_host_disk"]
   # Tag and field conversions defined in a separate sub-tables
  [[processors.regex.tags]]
    ## Tag to change
    key = "disk"
    ## Regular expression to match on a tag value
    # match disk tags that have numeric values
    pattern = "instance-total"
    ## Matches of the pattern will be replaced with this string.  Use ${1}
    ## notation to use the text of the first submatch.
    #set to white space, empty string don't work
    replacement = " "

## Copy any non-blank disk tags to zdim_disk - order is important relative to previous regex
[[processors.regex]]
  order=2
  namepass = ["vsphere_host_disk"]
   # Tag and field conversions defined in a separate sub-tables
  [[processors.regex.tags]]
    ## Tag to change
    key = "disk"
    ## Regular expression to match on a tag value
    # match disk tags that have numeric values
    pattern = "(\\S+)"
    ## Matches of the pattern will be replaced with this string.  Use ${1}
    ## notation to use the text of the first submatch.
    #keep the whole value
    replacement = "${1}"
    ## If result_key is present, a new field will be created
    ## instead of changing existing field
    # set the value on a new tag
    result_key = "zdim_disk"

# Name for host disk
[[processors.template]]
  order=3
  namepass=["vsphere_host_disk"]
  #drop disk tag as zdim_interface is what we want
  tagexclude=["disk"]
  tag = "zname"
  template = '{{ .Tag "zdim_vcenter" }}/{{ .Tag "zdim_dcname" }}/{{ .Tag "zdim_host" }}{{if .Tag "zdim_disk" }}/{{ .Tag "zdim_disk" }}{{end}}'

###################################
# vsphere_host_net
###################################
## determine interface dimension - blank any interface name for instance-total 
## instance-total metrics belong to the VM not the interface entities
[[processors.regex]]
  order=1
  namepass = ["vsphere_host_net"]
   # Tag and field conversions defined in a separate sub-tables
  [[processors.regex.tags]]
    ## Tag to change
    key = "interface"
    ## Regular expression to match on a tag value
    # match interface tags that have numeric values
    pattern = "instance-total"
    ## Matches of the pattern will be replaced with this string.  Use ${1}
    ## notation to use the text of the first submatch.
    #keep the whole value
    replacement = " "

## Copy any non-blank interface tags to zdim_interface - order is important relative to previous regex
[[processors.regex]]
  order=2
  namepass = ["vsphere_host_net"]
   # Tag and field conversions defined in a separate sub-tables
  [[processors.regex.tags]]
    ## Tag to change
    key = "interface"
    ## Regular expression to match on a tag value
    # match interface tags that have numeric values
    pattern = "(\\S+)"
    ## Matches of the pattern will be replaced with this string.  Use ${1}
    ## notation to use the text of the first submatch.
    #keep the whole value
    replacement = "${1}"
    ## If result_key is present, a new field will be created
    ## instead of changing existing field
    # set the value on a new tag
    result_key = "zdim_interface"

## Name for host net
[[processors.template]]
  order=3
  namepass=["vsphere_host_net"]
  #drop interface tag as zdim_interface is what we want
  tagexclude=["interface"]
  tag = "zname"
  template = '{{ .Tag "zdim_vcenter" }}/{{ .Tag "zdim_dcname" }}/{{ .Tag "zdim_host" }}{{if .Tag "zdim_interface" }}/{{ .Tag "zdim_interface" }}{{end}}'

###################################
# vsphere_host_storageAdapter
###################################
# Determine vsphere_host_storageAdapter  dimensions
[[processors.rename]]
  order=1
  namepass = ["vsphere_host_storageAdapter"]
  [[processors.rename.replace]]
    tag = "adapter"
    dest = "zdim_adapter"
# Name for vsphere_host_storageAdapter
[[processors.template]]
  order=2
  namepass=["vsphere_host_storageAdapter"]
  tagexclude=["cpu"]
  tagpass=["zdim_cpu"]
  tag = "zname"
  template = '{{ .Tag "zdim_vcenter" }}/{{ .Tag "zdim_dcname" }}/{{ .Tag "zdim_host" }}/{{ .Tag "zdim_adapter" }}'

#########################################
#        Zenoss vsphere vms             #
#########################################
#########################################
# Zenoss shared processor for vsphere vms
#########################################
[[processors.template]]
  order=0
  namepass=["vsphere_vm_mem", "vsphere_vm_sys", "vsphere_vm_power", "vsphere_vm_cpu", "vsphere_vm_virtualDisk", "vsphere_vm_net"]
  tag = "zdim_vm"
  template = '{{ .Tag "vmname" }}'
[[processors.rename]]
  order=0
  namepass=["vsphere_vm_mem", "vsphere_vm_sys", "vsphere_vm_power", "vsphere_vm_cpu", "vsphere_vm_virtualDisk", "vsphere_vm_net"]
  tagexclude=['host']
  ## Specify one sub-table per rename operation.
  [[processors.rename.replace]]
    tag = "vcenter"
    dest = "zdim_vcenter"
  [[processors.rename.replace]]
    tag = "dcname"
    dest = "zdim_dcname"

# Impact to vm host or vm depending on type
[[processors.template]]
  order=200
  namepass=["vsphere_vm_mem", "vsphere_vm_sys", "vsphere_vm_power", "vsphere_vm_cpu", "vsphere_vm_virtualDisk", "vsphere_vm_net"]
  tag = "impactToDimensions"
  # template = 'vcenter={{ .Tag "zdim_vcenter" }},dcname={{ .Tag "zdim_dcname" }},host={{ .Tag "esxhostname" }}'
  # template = 'vcenter={{ .Tag "zdim_vcenter" }},dcname={{ .Tag "zdim_dcname" }},{{if eq (.Tag "type") "vsphere.vm" }}host={{ .Tag "esxhostname" }}{{else}}vm={{ .Tag "zdim_vm" }}{{end}}'
  # template = '{{if eq (.Tag "type") "vsphere.vm" }}host={{ .Tag "esxhostname" }}{{else}}vm={{ .Tag "zdim_vm" }}{{end}}'
  template = '''source=zenoss.telegraf.truffle.local,vcenter={{ .Tag "zdim_vcenter" }},dcname={{ .Tag "zdim_dcname" }}
                {{- if eq (.Tag "type") "vsphere.vm" -}}
                  ,host={{ .Tag "esxhostname" }}
                {{- else -}}
                  ,vm={{ .Tag "zdim_vm" }}
                {{- end -}}
              '''


#procssor for vm and sub types
[[processors.template]]
  # determine types for cpu and subtypes
  #needs to be last processor for vsphere_host metrics
  order=100
  namepass=["vsphere_vm_mem", "vsphere_vm_sys", "vsphere_vm_power", "vsphere_vm_cpu", "vsphere_vm_virtualDisk", "vsphere_vm_net"]
  tag = "type"
  template = 'vsphere.{{if .Tag "zdim_cpu" }}vcpu{{else if .Tag "zdim_interface" }}vnic{{else if .Tag "zdim_disk" }}vdisk{{else}}vm{{end}}'

## Zenoss processor for vsphere_vm_mem vsphere_vm_sys vsphere_vm_power
# Name for vm as these are part of the vm model
[[processors.template]]
  order=1
  namepass=["vsphere_vm_mem", "vsphere_vm_sys", "vsphere_vm_power"]
  tag = "zname"
  template = '{{ .Tag "zdim_vcenter" }}/{{ .Tag "zdim_dcname" }}/{{ .Tag "zdim_vm" }}'

######################################
## Zenoss processor for vsphere_vm_cpu
######################################
# Determine vm cpu dimensions
[[processors.regex]]
  order=1
  namepass = ["vsphere_vm_cpu"]
   # Tag and field conversions defined in a separate sub-tables
  [[processors.regex.tags]]
    ## Tag to change
    key = "cpu"
    ## Regular expression to match on a tag value
    # match cpu tags that have numeric values - instance-total will be dropped as tag
    # since that is part of the vm entity not a cpu entity
    pattern = "([\\d])"
    ## Matches of the pattern will be replaced with this string.  Use ${1}
    ## notation to use the text of the first submatch.
    #keep the whole value
    replacement = "${1}"
    ## If result_key is present, a new field will be created
    ## instead of changing existing field
    # set the value on a new tag
    result_key = "zdim_cpu"

# Name for vm cpu
[[processors.template]]
  order=2
  namepass=["vsphere_vm_cpu"]
  tagexclude=["cpu"]
  tagpass=["zdim_cpu"]
  tag = "zname"
  template = '{{ .Tag "zdim_vcenter" }}/{{ .Tag "zdim_dcname" }}/{{ .Tag "zdim_vm" }}{{if .Tag "zdim_cpu" }}/{{ .Tag "zdim_cpu" }}{{end}}'

##############################################
## Zenoss processor for vsphere_vm_virtualDisk
##############################################
## determine disk dimension - blank any disk name for instance-total 
## instance-total metrics belong to the VM not the disk entities
[[processors.regex]]
  order=1
  namepass = ["vsphere_vm_virtualDisk"]
   # Tag and field conversions defined in a separate sub-tables
  [[processors.regex.tags]]
    ## Tag to change
    key = "disk"
    ## Regular expression to match on a tag value
    # match disk tags that have numeric values
    pattern = "instance-total"
    ## Matches of the pattern will be replaced with this string.  Use ${1}
    ## notation to use the text of the first submatch.
    #set to white space, empty string don't work
    replacement = " "

## Copy any non-blank disk tags to zdim_disk - order is important relative to previous regex
[[processors.regex]]
  order=2
  namepass = ["vsphere_vm_virtualDisk"]
   # Tag and field conversions defined in a separate sub-tables
  [[processors.regex.tags]]
    ## Tag to change
    key = "disk"
    ## Regular expression to match on a tag value
    # match disk tags that have numeric values
    pattern = "(\\S+)"
    ## Matches of the pattern will be replaced with this string.  Use ${1}
    ## notation to use the text of the first submatch.
    #keep the whole value
    replacement = "${1}"
    ## If result_key is present, a new field will be created
    ## instead of changing existing field
    # set the value on a new tag
    result_key = "zdim_disk"

# Name for vm disk
[[processors.template]]
  order=3
  namepass=["vsphere_vm_virtualDisk"]
  #drop disk tag as zdim_interface is what we want
  tagexclude=["disk"]
  tag = "zname"
  template = '{{ .Tag "zdim_vcenter" }}/{{ .Tag "zdim_dcname" }}/{{ .Tag "zdim_vm" }}{{if .Tag "zdim_disk" }}/{{ .Tag "zdim_disk" }}{{end}}'

######################################
## Zenoss processor for vsphere_vm_net
######################################
## determine interface dimension - blank any interface name for instance-total 
## instance-total metrics belong to the VM not the interface entities
[[processors.regex]]
  order=1
  namepass = ["vsphere_vm_net"]
   # Tag and field conversions defined in a separate sub-tables
  [[processors.regex.tags]]
    ## Tag to change
    key = "interface"
    ## Regular expression to match on a tag value
    # match interface tags that have numeric values
    pattern = "instance-total"
    ## Matches of the pattern will be replaced with this string.  Use ${1}
    ## notation to use the text of the first submatch.
    #keep the whole value
    replacement = " "

## Copy any non-blank interface tags to zdim_interface - order is important relative to previous regex
[[processors.regex]]
  order=2
  namepass = ["vsphere_vm_net"]
   # Tag and field conversions defined in a separate sub-tables
  [[processors.regex.tags]]
    ## Tag to change
    key = "interface"
    ## Regular expression to match on a tag value
    # match interface tags that have numeric values
    pattern = "(\\S+)"
    ## Matches of the pattern will be replaced with this string.  Use ${1}
    ## notation to use the text of the first submatch.
    #keep the whole value
    replacement = "${1}"
    ## If result_key is present, a new field will be created
    ## instead of changing existing field
    # set the value on a new tag
    result_key = "zdim_interface"

## Name for vm net
[[processors.template]]
  order=3
  namepass=["vsphere_vm_net"]
  #drop interface tag as zdim_interface is what we want
  tagexclude=["interface"]
  tag = "zname"
  template = '{{ .Tag "zdim_vcenter" }}/{{ .Tag "zdim_dcname" }}/{{ .Tag "zdim_vm" }}{{if .Tag "zdim_interface" }}/{{ .Tag "zdim_interface" }}{{end}}'
`