#!/bin/bash
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

function version_gt() { test "$(echo "$@" | tr " " "\n" | sort -V | head -n 1)" != "$1"; }
function version_le() { test "$(echo "$@" | tr " " "\n" | sort -V | head -n 1)" == "$1"; }
function version_lt() { test "$(echo "$@" | tr " " "\n" | sort -rV | head -n 1)" != "$1"; }
function version_ge() { test "$(echo "$@" | tr " " "\n" | sort -rV | head -n 1)" == "$1"; }


##Necessary software installation
function install_base_depenencies(){
    ## 1. set to update /etc/apt/sources.list
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  1.0   set to update /etc/apt/sources.list' ${NC}

    # 1.1 backup sources.list
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  1.1   backup /etc/apt/sources.list' ${NC}
    sudo mv /etc/apt/sources.list /etc/apt/sources.list.old.$(date '+%Y%m%d%H%M%S')

    # 1.2 create new sources.list , with 777 privileges
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  1.2   create new sources.list , with 777 privileges' ${NC}
    sudo touch /etc/apt/sources.list
    sudo chmod 777 /etc/apt/sources.list

    # 1.3 set the aliyun sources 
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  1.3   set the  sources  for registry' ${NC}
    if [ $1 = "cn" ]; then
        echo "deb http://mirrors.aliyun.com/ubuntu/ $(lsb_release -cs) main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb http://mirrors.aliyun.com/ubuntu/ $(lsb_release -cs)-security main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb http://mirrors.aliyun.com/ubuntu/ $(lsb_release -cs)-updates main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb http://mirrors.aliyun.com/ubuntu/ $(lsb_release -cs)-proposed main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb http://mirrors.aliyun.com/ubuntu/ $(lsb_release -cs)-backports main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb-src http://mirrors.aliyun.com/ubuntu/ $(lsb_release -cs) main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb-src http://mirrors.aliyun.com/ubuntu/ $(lsb_release -cs)-security main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb-src http://mirrors.aliyun.com/ubuntu/ $(lsb_release -cs)-updates main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb-src http://mirrors.aliyun.com/ubuntu/ $(lsb_release -cs)-proposed main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb-src http://mirrors.aliyun.com/ubuntu/ $(lsb_release -cs)-backports main restricted universe multiverse" >> /etc/apt/sources.list
    else
        echo "deb http://archive.ubuntu.com/ubuntu/ $(lsb_release -cs) main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb http://archive.ubuntu.com/ubuntu/ $(lsb_release -cs)-security main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb http://archive.ubuntu.com/ubuntu/ $(lsb_release -cs)-updates main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb http://archive.ubuntu.com/ubuntu/ $(lsb_release -cs)-proposed main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb http://archive.ubuntu.com/ubuntu/ $(lsb_release -cs)-backports main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb-src http://archive.ubuntu.com/ubuntu/ $(lsb_release -cs) main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb-src http://archive.ubuntu.com/ubuntu/ $(lsb_release -cs)-security main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb-src http://archive.ubuntu.com/ubuntu/ $(lsb_release -cs)-updates main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb-src http://archive.ubuntu.com/ubuntu/ $(lsb_release -cs)-proposed main restricted universe multiverse" >> /etc/apt/sources.list
        echo "deb-src http://archive.ubuntu.com/ubuntu/ $(lsb_release -cs)-backports main restricted universe multiverse" >> /etc/apt/sources.list
    fi
    ## 1.4 update apt-get
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  1.4.   sudo apt-get update' ${NC}
    sudo apt-get update || 
    if [ $? -ne 0 ]; then
        echo -e ${RED} "1.4.   sudo apt-get update failed" ${NC}  && exit 
    fi
    ## 1.5 upgrade apt-get
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  1.5.   sudo apt-get upgrade' ${NC}
    sudo apt-get upgrade -y 
    if [ $? -ne 0 ]; then
        echo -e ${RED} "1.5.   sudo apt-get upgrade failed" ${NC}  && exit 
    fi

    ## 1.6 install build-essential 
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  1.6.   sudo apt-get install build-essential' ${NC}
    sudo apt-get -y install   build-essential 
    if [ $? -ne 0 ]; then
        echo -e ${RED} "1.6.   sudo apt-get install build-essential failed" ${NC}  && exit 
    fi

    ## 1.7 install libssl-dev 
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  1.7.   sudo apt-get install  libssl-dev' ${NC}
    sudo apt-get -y install   libssl-dev  
    if [ $? -ne 0 ]; then
        echo -e ${RED} "1.7.   sudo apt-get install  libssl-dev failed" ${NC}  && exit 
    fi

    ## 1.8 install dkms
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  1.8.   sudo apt-get install  dkms' ${NC}
    sudo apt-get  -y install dkms 
    if [ $? -ne 0 ]; then
        echo -e ${RED} "1.8.   sudo apt-get install  dkms failed" ${NC}  && exit 
    fi

    ## 1.9 install git 
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  1.9.   sudo apt-get  install  git' ${NC}
    sudo apt-get -y install  git 
    ## 1.10 install jq 
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  1.10.   sudo apt-get  install  jq' ${NC}
    sudo apt-get -y install  jq 
    ## 1.11 install curl
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  1.10.   sudo apt-get  install  curl' ${NC}
    sudo apt-get -y install  curl 
     ## 1.11 install wget
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  1.10.   sudo apt-get  install  wget' ${NC}
    sudo apt-get -y install  wget 

}

 ## 2.0 sgx driver installation
function install_sgx_env(){
    # Check the kernel version. For version 5.11 or above, the system has the sgx driver installed by default.
    VERSION=$(uname -r | awk -F '-' '{print $1}')
     if version_lt $VERSION "5.11.0"; then #Ubuntu kernel version 5.11 and later comes with the sgx driver, no additional installation is required.
      echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  2.0   download and  install sgx_linux_x64_driver_1.41.bin' ${NC}
        if [ ! -x /opt/intel/sgxdriver ]; then
            sudo rm -rf sgx_linux_x64_driver_1.41.bin
            FILE=sgx_linux_x64_driver_1.41.bin
            if [[ ! -f "$FILE" ]]; then
                wget https://download.01.org/intel-sgx/sgx-dcap/1.12/linux/distro/ubuntu20.04-server/sgx_linux_x64_driver_1.41.bin
            fi
            result=$(echo '381c32da43ad500bac104601341c8f53f63e4e6f507259b463fa920b3e67bc4f sgx_linux_x64_driver_1.41.bin' | sha256sum -c | grep -e 'OK' -e '成功')
            if [[ $result = "" ]]; then 
            echo -e ${RED} "2.0. sgx_linux_x64_driver_1.41.bin checksum failed" ${NC} && exit  ${NC}
            exit 
            fi
            chmod +x sgx_linux_x64_driver_1.41.bin
            sudo ./sgx_linux_x64_driver_1.41.bin
            if [ $? -ne 0 ]; then
                echo -e ${RED} "2.0  install sgx_linux_x64_driver_1.41.bin failed" ${NC}  && exit 
            fi
            sudo rm -rf sgx_linux_x64_driver_1.41.bin

        else
            echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  2.0  ******* sgx_driver aleady installed ********' ${NC}
        fi
    fi
    ## 2.1 queto provider installation
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  2.1   apt-key add intel-sgx-deb.key' ${NC}
    FILE=intel-sgx-deb.key
    if [[ ! -f "$FILE" ]]; then
        wget --no-check-certificate  https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key
    fi
    sudo apt-key add intel-sgx-deb.key
    echo "deb [arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/intel-sgx.list
    sudo apt-get update -y 
    if [ $? -ne 0 ]; then
        echo -e ${RED} "apt-get update failed" ${NC}  && exit 
    fi
    sudo rm -rf intel-sgx-deb.key
    sudo apt-get install -y  libsgx-enclave-common libsgx-dcap-ql 
    ## 2.2 libsgx-dcap-default-qpl installation
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  2.2   download and  install libsgx-dcap-default-qpl' ${NC}
    sudo apt-get install libsgx-dcap-default-qpl 
    if [ $? -ne 0 ]; then
        echo -e ${RED} "2.2   download and  install libsgx-dcap-default-qpl failed" ${NC} && exit 
    fi
    FILE=/usr/lib/x86_64-linux-gnu/libdcap_quoteprov.so
    if [[  -f "$FILE" ]]; then
        sudo rm -rf /usr/lib/x86_64-linux-gnu/libdcap_quoteprov.so
    fi
    sudo ln -s /usr/lib/x86_64-linux-gnu/libdcap_quoteprov.so.1 /usr/lib/x86_64-linux-gnu/libdcap_quoteprov.so
    if [ $? -ne 0 ]; then
        echo -e ${RED} "2.2 create soft link for /usr/lib/x86_64-linux-gnu/libdcap_quoteprov.so  failed: $? " ${NC}  && exit 
        exit
    fi

    ## 2.3 config /etc/sgx_default_qcnl.conf
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  2.3   config /etc/sgx_default_qcnl.conf' ${NC}
    sudo bash -c 'echo { > /etc/sgx_default_qcnl.conf'
    sudo bash -c 'echo  \"pccs_url\": \"https://localhost:8081/sgx/certification/v4/\", >> /etc/sgx_default_qcnl.conf'
    sudo bash -c 'echo  \"use_secure_cert\": false, >> /etc/sgx_default_qcnl.conf'
    sudo bash -c 'echo  \"pccs_api_version\": \"3.1\", >> /etc/sgx_default_qcnl.conf'
    sudo bash -c 'echo  \"retry_times\": 6, >> /etc/sgx_default_qcnl.conf'
    sudo bash -c 'echo  \"retry_delay\": 10, >> /etc/sgx_default_qcnl.conf'
    sudo bash -c 'echo  \"pck_cache_expire_hours\": 168, >> /etc/sgx_default_qcnl.conf'
    sudo bash -c 'echo  \"verify_collateral_cache_expire_hours\": 168 >> /etc/sgx_default_qcnl.conf'
    sudo bash -c 'echo  } >> /etc/sgx_default_qcnl.conf'
    if [ $? -ne 0 ]; then
        echo -e ${RED} "2.3   config /etc/sgx_default_qcnl.conf failed" ${NC}  && exit 
    fi
    ## 2.4  libsgx-enclave-common installation
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  2.4   download and  install libsgx-enclave-common' ${NC}
    sudo apt-get -y install  --no-install-recommends libsgx-enclave-common 
    if [ $? -ne 0 ]; then
        echo -e ${RED} "2.4   download and  install libsgx-enclave-common failed" ${NC}  && exit 
    fi
    ## 2.5  libsgx-quote-ex installation
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  2.5   download and  install libsgx-quote-ex' ${NC}
    sudo apt-get -y install libsgx-quote-ex 
    if [ $? -ne 0 ]; then
        echo -e ${RED} "2.5   download and  install libsgx-quote-ex failed" ${NC}  && exit 
    fi
  
    if version_lt   $VERSION "5.11.0"; then
        ## 2.6 Check whether the sgx related drivers and software are installed successfully
        echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  2.6   check if sgx env init success' ${NC}
        /sbin/modprobe intel_sgx
        if [ $? -ne 0 ]; then
            echo -e ${RED} "2.6   modprobe intel_sgx failed,please confirm that sgx in the bios has been correctly enabled" ${NC}  && exit 
        fi
        echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  2.6   sgx env init success' ${NC}
    fi
    ## 2.7 Install sgx pck id retrieval tool
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  2.7   download and  install sgx-pck-id-retrieval-tool' ${NC}
    sudo apt-get -y install sgx-pck-id-retrieval-tool 
    if [ $? -ne 0 ]; then
        echo -e ${RED} "2.7   download and  install sgx-pck-id-retrieval-tool failed" ${NC} 
    else
        echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  2.7   download and  install sgx-pck-id-retrieval-tool success' ${NC}
        # Update configuration
        sudo sed -i "s/#PCCS_URL=https:\/\/localhost:8081\/sgx\/certification\/v4\/platforms/PCCS_URL=https:\/\/localhost:8081\/sgx\/certification\/v4\/platforms/"  /opt/intel/sgx-pck-id-retrieval-tool/network_setting.conf
        sudo sed -i "s/#USE_SECURE_CERT=TRUE/USE_SECURE_CERT=FALSE/"  /opt/intel/sgx-pck-id-retrieval-tool/network_setting.conf
        sudo sed -i "s/#user_token =[ ]*/user_token =\$Dcnetio_user0\$/"  /opt/intel/sgx-pck-id-retrieval-tool/network_setting.conf
        sudo sed -i "s/#proxy_type[ ]*= direct/proxy_type    = direct/"  /opt/intel/sgx-pck-id-retrieval-tool/network_setting.conf
    fi  
    
}

## 3.0 docker installation
function install_docker(){
    #remove  docker installed by snap, if any.Because snap docker will cause file not exist error when run docker on bind mount
    sudo snap remove docker
    #docker_installation
    if [ ! -x /var/lib/docker ]; then
        echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  3.0   INSTALLING docker' ${NC}
        echo
        echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  3.1   install docker dependencies' ${NC}
                sudo apt-get install apt-transport-https ca-certificates curl software-properties-common -y
        echo
        echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  3.2   add docker’s official GPG key' ${NC}
                curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
        echo
        echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  3.3   set up the stable repository' ${NC}
            sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"	
        echo
        echo
        echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  3.4   update the packages' ${NC}
            sudo apt-get update -y
        echo
        echo
        echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  3.5   checks install from the Docker repo instead of the default Ubuntu repo' ${NC}
            sudo apt-cache policy docker-ce 
        echo
        echo
        echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  3.6   installing dcoker through docker-ce' ${NC}
            sudo apt-get install docker-ce -y 
        echo
        echo
        echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  3.7   installing docker-compose' ${NC}
            sudo apt-get install docker-compose -y
        echo
        echo
        echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  3.8   adds your username to the docker group'  ${NC}
            sudo usermod -a -G docker ${USER} 
            name=$(who am i | awk '{print $1}')
            sudo usermod -a -G docker $name
        echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  3.9   docker successfully installed' ${NC}
        echo
        echo
    else
       #Determine whether docker compose already exists, if not, install it
        if ! [ -x "$(command -v docker-compose)" ]; then
            echo
            echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  3.4   update the packages' ${NC}
                sudo apt-get update -y
            echo
            echo
            echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  3.7   installing docker-compose' ${NC}
                sudo apt-get install docker-compose -y
            echo
            echo
        fi
        echo
        echo
                echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  3.9  DOCKER ALREADY INSTALLED'${NC}
        echo
        echo
    fi
    #Configure Docker to start on boot with systemd
    systemctl enable docker.service
    systemctl enable containerd.service
    #if docker is not running, start docker
    if ! systemctl is-active --quiet docker.service; then
        systemctl start docker.service
    fi
   #if containerd is not running, start containerd
    if ! systemctl is-active --quiet containerd.service; then
        systemctl start containerd.service
    fi
}



newest_docker_tag="latest"

#get docker tag list from ghcr.io
function get_docker_newesttag_list(){
   
   #get  fake NOOP token from github
   noop_token=$(curl -s https://ghcr.io/token\?scope\="repository:$1:pull" | jq -r .token)
   docker_tag_list=$(curl -H "Authorization: Bearer $noop_token" https://ghcr.io/v2/$1/tags/list  | jq -r '.tags[]')
if [ $? -ne 0 ]; then
        echo -e ${RED} " get docker tag list from ghcr.io failed" ${NC}  && exit 
   fi
   #get newest docker tag
   newest_docker_tag=$(echo $docker_tag_list | awk '{print $NF}')
   if newest_docker_tag=""; then # get docker tag list from tags.dcnetio.cloud
    newest_docker_tag=$(curl -s https://ghcr.dcnetio.cloud/$1)
   fi
}




## 4.0 Download docker image
function install_docker_images(){
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  4.0   get dcnetio/pccs newest tag' ${NC}
    get_docker_newesttag_list dcnetio/pccs
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') ">>  4.1   pull image from ghcr.io/dcnetio/pccs:${newest_docker_tag}" ${NC}
    echo
    docker pull ghcr.io/dcnetio/pccs:$newest_docker_tag
    if [ $? -ne 0 ]; then
        echo -e ${RED} "4.2   pull image from ghcr.io/dcnetio/pccs failed" ${NC}  && exit
    fi
    sudo sed -i "s/pccsImage:.*/pccsImage: ghcr.io\/dcnetio\/pccs:${newest_docker_tag}/" $1/manage_config.yaml
    echo
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  4.3   get dcnetio/dcchain newest tag' ${NC}
    get_docker_newesttag_list dcnetio/dcchain
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') ">>  4.4   pull image from ghcr.io/dcnetio/dcchain:${newest_docker_tag}" ${NC}
    echo
    docker pull ghcr.io/dcnetio/dcchain:$newest_docker_tag
    if [ $? -ne 0 ]; then
        echo -e ${RED} "4.5   pull image from ghcr.io/dcnetio/dcchain failed" ${NC}  && exit 
    fi
    sudo sed -i "s/chainImage:.*/chainImage: ghcr.io\/dcnetio\/dcchain:${newest_docker_tag}/" $1/manage_config.yaml
    echo
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  4.6   get dcnetio/dcupgrade newest tag' ${NC}
    get_docker_newesttag_list dcnetio/dcupgrade
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') ">>  4.7   pull image from ghcr.io/dcnetio/dcupgrade:${newest_docker_tag}" ${NC}
    echo
    docker pull ghcr.io/dcnetio/dcupgrade:$newest_docker_tag
    if [ $? -ne 0 ]; then
        echo -e ${RED} "4.8   pull image from ghcr.io/dcnetio/dcupgrade failed" ${NC}  && exit 
    fi
    sudo sed -i "s/upgradeImage:.*/upgradeImage: ghcr.io\/dcnetio\/dcupgrade:${newest_docker_tag}/" $1/manage_config.yaml
    echo
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  4.9   get dcnetio/dcstorage newest tag' ${NC}
    get_docker_newesttag_list dcnetio/dcstorage
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') ">>  5.0   pull image from ghcr.io/dcnetio/dcstorage:${newest_docker_tag}" ${NC}
    echo
    docker pull ghcr.io/dcnetio/dcstorage:$newest_docker_tag
    if [ $? -ne 0 ]; then
        echo -e ${RED} "4.3   pull image from ghcr.io/dcnetio/dcstorage failed" ${NC}  && exit 
    fi
    sudo sed -i "s/nodeImage:.*/nodeImage: ghcr.io\/dcnetio\/dcstorage:${newest_docker_tag}/" $1/manage_config.yaml
    echo

}


## 4.0-cn Download docker image from registry.cn-hangzhou.aliyuncs.com/dcproxy
function install_docker_images_cn(){
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  4.0   get dcnetio/pccs newest tag' ${NC}
    get_docker_newesttag_list dcnetio/pccs
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') ">>  4.1   pull image from registry.cn-hangzhou.aliyuncs.com/dcproxy/pccs:${newest_docker_tag}" ${NC}
    echo
    docker pull registry.cn-hangzhou.aliyuncs.com/dcproxy/pccs:$newest_docker_tag
    if [ $? -ne 0 ]; then
        echo -e ${RED} "4.2   pull image from registry.cn-hangzhou.aliyuncs.com/dcproxy/pccs failed" ${NC}  && exit
    fi
    sudo sed -i "s/pccsImage:.*/pccsImage: registry.cn-hangzhou.aliyuncs.com\/dcproxy\/pccs:${newest_docker_tag}/" $1/manage_config.yaml
    echo
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  4.3   get dcnetio/dcchain newest tag' ${NC}
    get_docker_newesttag_list dcnetio/dcchain
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') ">>  4.4   pull image from registry.cn-hangzhou.aliyuncs.com/dcproxy/dcchain:${newest_docker_tag}" ${NC}
    echo
    docker pull registry.cn-hangzhou.aliyuncs.com/dcproxy/dcchain:$newest_docker_tag
    if [ $? -ne 0 ]; then
        echo -e ${RED} "4.5   pull image from registry.cn-hangzhou.aliyuncs.com/dcproxy/dcchain failed" ${NC}  && exit 
    fi
    sudo sed -i "s/chainImage:.*/chainImage: registry.cn-hangzhou.aliyuncs.com\/dcproxy\/dcchain:${newest_docker_tag}/" $1/manage_config.yaml
    echo
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  4.6   get dcnetio/dcupgrade newest tag' ${NC}
    get_docker_newesttag_list dcnetio/dcupgrade
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') ">>  4.7   pull image from registry.cn-hangzhou.aliyuncs.com/dcproxy/dcupgrade:${newest_docker_tag}" ${NC}
    echo
    docker pull registry.cn-hangzhou.aliyuncs.com/dcproxy/dcupgrade:$newest_docker_tag
    if [ $? -ne 0 ]; then
        echo -e ${RED} "4.8   pull image from registry.cn-hangzhou.aliyuncs.com/dcproxy/dcupgrade failed" ${NC}  && exit 
    fi
    sudo sed -i "s/upgradeImage:.*/upgradeImage: registry.cn-hangzhou.aliyuncs.com\/dcproxy\/dcupgrade:${newest_docker_tag}/" $1/manage_config.yaml
    echo
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') '>>  4.9   get dcnetio/dcstorage newest tag' ${NC}
    get_docker_newesttag_list dcnetio/dcstorage
    echo -e ${GREEN} $(date '+%Y-%m-%d %H:%M:%S') ">>  5.0   pull image from registry.cn-hangzhou.aliyuncs.com/dcproxy/dcstorage:${newest_docker_tag}" ${NC}
    echo
    docker pull registry.cn-hangzhou.aliyuncs.com/dcproxy/dcstorage:$newest_docker_tag
    if [ $? -ne 0 ]; then
        echo -e ${RED} "4.3   pull image from registry.cn-hangzhou.aliyuncs.com/dcproxy/dcstorage failed" ${NC}  && exit 
    fi
    sudo sed -i "s/nodeImage:.*/nodeImage: registry.cn-hangzhou.aliyuncs.com\/dcproxy\/dcstorage:${newest_docker_tag}/" $1/manage_config.yaml
    echo

}





