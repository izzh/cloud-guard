#!/bin/bash
set SPECIFIED_REGION=127.0.0.1:6751

agent_pkg=elkeid-agent-1.0.2-1.x86_64.rpm

 yum remove elkeid-agent
 rpm -ivh $agent_pkg

 systemctl start elkeid-agent
