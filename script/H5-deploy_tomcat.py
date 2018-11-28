# -*- coding:utf8 -*-
import subprocess
import json
import sys
import os

# python3 -u H5-deploy_tomcat.py hi-aliyun-prod/
if __name__ == "__main__":
    if (len(sys.argv) < 2):
        print("Usage: python3 H5-deploy_tomcat.py [project name]")
        sys.exit()
    
    # copy setting files to current working directory
    subprocess.call(["cp", "-R", sys.argv[1] + "_settings/.", "."])

    with open("settings.json") as config_file:
        config = json.load(config_file)
    
    project = config["project"]

    # clone project from git
    if (len(sys.argv) == 3):
        if (sys.argv[2] == "recover"):
            # 调用 shell 命令行 deployer 程序
            subprocess.call(["deployer", "-config", "settings.json", "-recover", "true"])
            print ("recover project.")
            sys.exit()

        else:
            subprocess.call(["git", "clone", "-b", sys.argv[2], "git@gitlab-root:root/%s.git" % project])
            print ("new config")

    else:
        subprocess.call(["git", "clone", "git@gitlab-root:root/%s.git" % project])
        print ("old config")

    # 调用 shell 命令行 deployer 程序
    subprocess.call(["deployer", "-config", "settings.json"])