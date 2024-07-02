#!/bin/bash

localbasedir=$(cd `dirname $0`;pwd)
localscriptdir=$localbasedir/scripts
localbindir=$localbasedir/bin
installdir=/opt/dcnetio
installbindir=$installdir/bin
installetcdir=$installdir/etc
datadir=$installdir/data
disksdir=$installdir/disks
uninstallscript=$localscriptdir/uninstall.sh
source $localscriptdir/init.sh
region="en"

USERNAME=$(getent passwd `who` | head -n 1 | cut -d : -f 1)
help()
{
cat << EOF
Usage:
    --registry {cn|en}       use registry to accelerate apt-get install  in some areas
EOF
exit 0
}



if [ $(id -u) -ne 0 ]; then
    echo -e ${RED} "Please run with sudo!" ${NC} && exit 
fi

# create usr dcnetio that be locked out of logging in
if [ ! -d /home/dcnetio ]; then
    sudo useradd -m -s /bin/bash dcnetio
    sudo passwd -l dcnetio
fi

# create usrgroup dcnetio, and add usr dcnetio to it
if [ ! $(getent group dcnetio) ]; then
    sudo groupadd dcnetio
    sudo usermod -a -G dcnetio dcnetio
fi

# add current usr to dcnetio group
sudo usermod -a -G dcnetio $USERNAME




while true ; do
    case "$1" in
        --registry)
            if [ x"$2" == x"" ] || [[ x"$2" != x"cn" && x"$2" != x"en" ]]; then
                help
            fi
            region=$2
            shift 2
            break ;;
        --help)
            help
            break ;;
        *)
            echo -e ${GREEN} "如果安装速度很慢，尝试使用 'sudo ./install.sh --registry cn' 命令来加快安装速度" ${NC}
            break;
            ;;
    esac
done



BEGINTIME=$(date "+%Y-%m-%d %H:%M:%S")
echo $BEGINTIME '>>  start dc  node install...'
#set needrestart
sed -i "/#\$nrconf{restart} = 'i';/s/.*/\$nrconf{restart} = 'a';/" /etc/needrestart/needrestart.conf
result=$(sudo cat /etc/sudoers |grep $USERNAME| grep 'ALL=(ALL:ALL)')
if [[ $result = "" ]]; then
    sudo chmod +w /etc/sudoers
    sudo echo $USERNAME "     ALL=(ALL:ALL) ALL" >> /etc/sudoers
    sudo chmod -w /etc/sudoers
fi

sudo cat  /etc/sudoers | grep $USERNAME
if [ ! -d $installdir ]; then
    sudo mkdir -p $installdir
fi
if [ ! -d $installbindir ]; then
    sudo mkdir -p $installbindir
fi
if [ ! -d $installetcdir ]; then
    sudo mkdir -p $installetcdir
fi
if [ ! -d $datadir ]; then
    sudo mkdir -p $datadir
fi
if [ ! -d $disksdir ]; then
    sudo mkdir -p $disksdir
fi
if [ ! -f $installdir/log ]; then
    sudo touch $installdir/log
fi
sudo cp -rf $localbindir/* $installbindir
sudo cp -rf $uninstallscript $installbindir
# Determine whether it is the first installation. If it is the first installation, you need to copy the configuration file.
if [  -f $installetcdir/manage_config.yaml ]; then
    # Ask the user if they need to overwrite the configuration file
    read -p "Do you want to overwrite the configuration file? [y/n] " answer
    if [ $answer = "y" ]; then
        sudo cp -rf $localbasedir/etc/* $installetcdir
    else
        sudo cp -rf $localbasedir/etc/dc.bash_completion $installetcdir
        sudo cp -rf $localbasedir/etc/dc.service $installetcdir
    fi
else
    sudo cp -rf $localbasedir/etc/* $installetcdir
fi


#set image registry
if [ $region  = "cn" ]; then
    sudo sed -i "s/registry:.*/registry: cn/" $installetcdir/manage_config.yaml
else
    sudo sed -i "s/registry:.*/registry: en/" $installetcdir/manage_config.yaml
fi
   
sudo chmod +x $installbindir/*
sudo ln -s $installbindir/dc   /usr/bin/dc



install_base_depenencies $region
install_sgx_env

if [ $region = "cn" ]; then
    install_docker_cn
    install_docker_images_cn $installetcdir
else
    install_docker
    install_docker_images  $installetcdir
fi
#set command completion
sudo cp $installetcdir/dc.bash_completion /etc/bash_completion.d/dc
sudo chmod +x /etc/bash_completion.d/dc
#set systemd service
sudo cp $installetcdir/dc.service /etc/systemd/system/dc.service
sudo chmod +x /etc/systemd/system/dc.service
sudo systemctl daemon-reload
sudo systemctl enable dc.service
sudo systemctl start dc.service

# change the owner of the installdir to dcnetio
sudo chown -R dcnetio:dcnetio $installdir
# change the permission of the installdir to 775
sudo chmod 775 $installdir -R
ENDTIME=$(date "+%Y-%m-%d %H:%M:%S")
echo $ENDTIME '>>  end dc  node install...'
start_seconds=$(date --date="$BEGINTIME" +%s);
end_seconds=$(date --date="$ENDTIME" +%s);
echo "install time : "$((end_seconds-start_seconds))"s"
echo -e ${GREEN} "Please run '. /etc/bash_completion; newgrp docker; newgrp dcnetio' to enable command completion and refresh user group" ${NC}


