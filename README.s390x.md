This is a fork of the Openblockchain obc-peer repository which has been patched to compile on Linux s390x. Please follow the steps below to build OBC. The instructions assume a Debian host but should also work on Ubuntu. When appropriate, commands for RHEL or SLES are also given.

####1. Install gcc6
Until native go compiler on Linux s390x is released, we have to use gccgo. OBC requires go1.5.1 and above, which is part of gcc6.
   
   ```
   # apt-get install libgmp-dev libmpfr-dev libmpc-dev zlib1g-dev libbz2-dev
   # git clone git://gcc.gnu.org/git/gcc.git
   # cd gcc
   # mkdir build
   # cd build
   # ../configure --disable-bootstrap --disable-multilib --prefix=/opt/gcc-6.0.0 --enable-shared --with-system-zlib --enable-threads=posix --enable-__cxa_atexit --enable-gnu-indirect-function --enable-languages=c,go,c++
   # make all & make install
   ```

   For RHEL, replace apt-get command with
   ```
   # yum install gmp-devel mpfr-devel libmpc-devel zlib-devel bzip2-devel
   ```
   
   For SLES, replace apt-get command with
   ```   
   # zypper install gmp-devel mpfr-devel mpc-devel zlib-devel libbz2-devel
   ```
   
This will install gcc6 into /opt/gcc-6.0.0. To use it, add /opt/gcc-6.0.0/bin to your PATH and /opt/gcc-6.0.0/lib64 to your LD_LIBRARY_PATH.
   
####2. Install rocksdb
The example below clones into /opt/rocksdb. You can choose wherever you prefer.

   ```
   # cd /opt
   # git clone https://github.com/facebook/rocksdb.git
   # cd rocksdb
   # make shared_lib
   ```
   
This generates librocksdb.so in /opt/rocksdb. For compiling code, add -I/opt/rocksdb/include to your compiler flag and -L/opt/rocksdb -lrocksdb to your linker flag. For running code, add /opt/rocksdb to your LD_LIBRARY_PATH.
   
####3. Install docker
For RHEL and SLES, you can get it at https://www.ibm.com/developerworks/linux/linux390/docker.html although it's recommended to use the more recent version that I compiled for Debian, which you can get it here at https://github.com/gongsu832/docker-s390x-debian (these are statically linked binaries so they should also work on RHEL and SLES). On Debian and Ubuntu, you may need to install aufs-tools if it's not already installed and you want to use aufs as the storage driver.

   ```
   apt-get install aufs-tools
   ```

While the docker daemon can automatically pull images on demand, it's better to pull the pre-built OBC image now since it's more than 1GB. To do that,

   ```
   # docker daemon -H tcp://0.0.0.0:4243 -H unix:///var/run/docker.sock > /var/log/docker.log 2>&1 &
   # docker pull s390xlinux/obc-gccgo1.6-rocksdb4.5
   ```
   
Note that -H tcp://0.0.0.0:4243 is not really needed for pulling images. It's for doing the OBC "behave" tests later.
   
####4. Build OBC
Once again, the example below uses GOPATH=/opt/openchain and you are free to use wherever you prefer.

   ```
   # apt-get install libsnappy-dev
   # cd /opt
   # mkdir -p openchain/src/github.com/openblockchain
   # cd openchain/src/github.com/openblockchain
   # git clone https://github.com/gongsu832/obc-peer.git
   # cd obc-peer
   # export GOPATH=/opt/openchain
   # ./s390xVendor.sh
   # CGO_CFLAGS="-I/opt/rocksdb/include" CGO_LDFLAGS="-L/opt/rocksdb -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" go build
   ```
   
   For RHEL, replace apt-get command with
   ```
   # yum install snappy-devel
   ```
   
   For SLES, replace apt-get command with
   ```
   # zypper install snappy-devel
   ```
   
This generates obc-peer binary in the current directory. Note you must run the s390xVendor.sh script before building. It relocates the vendor subdirectories to the original places under $GOPATH/src. Because gccgo does not support vendor subdirectories yet.

To build the OBC CA server, which is required when security is enabled,

   ```
   # cd obc-ca
   # CGO_CFLAGS="-I/opt/rocksdb/include" CGO_LDFLAGS="-L/opt/rocksdb -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" go build -o obcca-server
   ```

#####To run the unit tests,
   
   ```
   # ./obc-peer peer > /var/log/obc-peer.log 2>&1 &
   # CGO_CFLAGS="-I/opt/rocksdb/include" CGO_LDFLAGS="-L/opt/rocksdb -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" go test -timeout=20m $(go list github.com/openblockchain/obc-peer/...|grep -v /examples/)
   ```
   
You should have no FAIL for any test. Note that to rerun the tests, you need to clean up all the leftover containers and images except s390xlinux/obc-gccgo1.6-rocksdb4.5.
   
#####To run the behave tests,

Install docker-compose and behave if you haven't already,

   ```
   # apt-get install python-setuptools python-pip
   # pip install docker-compose behave
   ```

   For RHEL
   ```
   # yum install python-setuptools
   # curl "https://bootstrap.pypa.io/get-pip.py" -o "get-pip.py"
   # python get-pip.py
   # pip install docker-compose behave
   ```
   
   For SLES
   ```
   # zypper install python-devel python-setuptools
   # curl "https://bootstrap.pypa.io/get-pip.py" -o "get-pip.py"
   # python get-pip.py
   # pip install docker-compose behave
   ```
   
Then run the behave tests,

   ```
   # killall obc-peer
   # cd openchain/peer/bddtests
   # behave >behave.log 2>&1 &
   ```
   
You may have a few failed and skipped tests and it's OK to ignore them if they are not "cannot connect to Docker endpoint" error mentioned below. Note that to rerun the tests, you need to clean up all the leftover containers and images except s390xlinux/obc-gccgo1.6-rocksdb4.5, openchain-peer, and obcca.

If you have tests failing for reason like "cannot connect to Docker endpoint", check the IP address of interface docker0 on your system. In compose-defaults.yml file, the IP address is assumed to be 172.17.0.1. You must change it to match what you have on your system.
