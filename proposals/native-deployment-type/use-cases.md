# Planned use cases

### 1. Security update
A security vulnerability is identified in an installed package such as containerd, and a patched version is available. The DFM shows which devices are still using an affected package version. The update to the approved version is started manually for all selected devices. Afterwards, the DFM displays the installation status and the resulting package version. 

### 2. New package version required
A newer approved version of a system library such as OpenSSL is required. The DFM shows which package version is installed on each device and which devices do not yet meet the desired version level. The update is started manually for all selected devices. Afterwards, the DFM displays the installation status and the new package version. 

### 3. Package rollback due to incompatibility
A Beckhoff system package such as mdp-bhf has been updated from version A to version B. It is then found that version B does not work as expected. The DFM shows which devices are already using version B. The rollback to the previously approved version A is started manually for all affected devices. Afterwards, the DFM displays the package version and device status. 

### 4. Installation of a completely new package
An additional system package such as nmap is to be made available on the devices. The DFM shows which devices do not yet have the package installed. Installation of the approved version is started manually for all selected devices or an entire device group. Afterwards, the DFM displays the installation status and the resulting package version. 

### 5. Package uninstallation
A previously required package is no longer used, for example because an application has been removed or a function has been replaced. The DFM shows which devices still have the package installed. Uninstallation is started manually for all selected devices or an entire device group. Afterwards, the DFM shows whether the package was successfully removed and which devices do not yet meet the desired state. 

### 6. Updating related packages together
A component consists of several interdependent packages. For the Docker stack, these include, for example, docker-ce, docker-ce-cli, containerd.io, docker-compose-plugin and docker-buildx-plugin. In the DFM, these packages are managed as a related package group or as a single deployment. The update to a defined set of mutually compatible versions is started manually for all selected devices or a device group. Afterwards, the DFM displays the installation status and the resulting package versions. 

### 7. Kernel/RT kernel update
A new approved version of the Beckhoff RT kernel, for example the linux-image-bhf package, is to be installed on the devices. The DFM shows which devices are still using an older kernel version. Installation is started manually for all selected devices or a device group. After installation, the required restart is carried out in a controlled manner. Afterwards, the DFM checks whether the devices are reachable again and whether the desired kernel version is active. 

### 8. Update failure
A package cannot be installed, for example due to an unmet dependency, insufficient storage space or an unreachable package source. The desired state is package B, but package A remains installed on the device and the update receives the status FAILED. The DFM shows that the device has not reached the desired state and provides the current package version and the error message for further analysis. 