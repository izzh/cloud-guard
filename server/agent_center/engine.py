import requests
import threading
import time
import json
from kafka import KafkaConsumer
from cachetools import TTLCache

# 创建线程安全的缓存, 过期时间为 5 分钟
alarm_cache = TTLCache(maxsize=100, ttl=300)

# 告警规则同步线程
def sync_alarm_rules():
    url = "http://127.0.0.1:7003/agent/alarm/strategy/allList"
    
    while True:
        try:
            response = requests.post(url)
            if response.status_code == 200:
                data = response.json().get('data', {})
                tenant_id = data.get('tenantId')
                for host in data.get('alarmStrategyHostList', []):
                    host_id = host.get('hostId')
                    for rule in host.get('alarmStrategyRuleList', []):
                        rule_id = rule.get('id')
                        strategy_id = rule.get('strategyId')
                        metric_type = rule.get('metricType')
                        operator = rule.get('operator')
                        trigger_value = int(rule.get('triggerValue'))

                        # 缓存规则，以 tenant_id, host_id, metric_type 为 key
                        cache_key = f"{tenant_id}_{host_id}_{metric_type}"
                        alarm_cache[cache_key] = {
                            "id": rule_id,
                            "tenantId": tenant_id,
                            "strategyId": strategy_id,
                            "hostId": host_id,
                            "metricType": metric_type,
                            "operator": operator,
                            "triggerValue": trigger_value
                        }
                print("Alarm rules synced and cached.")
            else:
                print("Failed to fetch alarm rules, status code:", response.status_code)
        except Exception as e:
            print("Error syncing alarm rules:", str(e))
        
        # 休眠2分钟
        time.sleep(120)

# 性能告警监控线程
def monitor_performance():
    consumer = KafkaConsumer(
        'hids_svr',
        bootstrap_servers=['127.0.0.1:9092'],
        auto_offset_reset='earliest',
        enable_auto_commit=True,
        group_id='performance_monitor'
    )
    
    alarm_count = {}

    for message in consumer:
        try:
            data = json.loads(message.value.decode('utf-8'))
            if data.get('data_type') == '1000':
                tenant_id = data.get('tenant_id')
                host_id = data.get('host_id')
                
                for metric in ['cpu_usage', 'mem_usage']:
                    metric_type = 'cpuUsage' if metric == 'cpu_usage' else 'memUsage'
                    metric_value = int(float(data.get(metric)) * 100)
                    
                    cache_key = f"{tenant_id}_{host_id}_{metric_type}"
                    if cache_key in alarm_cache:
                        rule = alarm_cache[cache_key]
                        operator = rule['operator']
                        trigger_value = rule['triggerValue']
                        
                        # 运算符解析和计算
                        trigger = False
                        if operator == 'eq' and metric_value == trigger_value:
                            trigger = True
                        elif operator == 'ne' and metric_value != trigger_value:
                            trigger = True
                        elif operator == 'gt' and metric_value > trigger_value:
                            trigger = True
                        elif operator == 'ge' and metric_value >= trigger_value:
                            trigger = True
                        elif operator == 'lt' and metric_value < trigger_value:
                            trigger = True
                        elif operator == 'le' and metric_value <= trigger_value:
                            trigger = True
                        
                        if trigger:
                            count_key = f"{tenant_id}_{host_id}_{metric_type}_count"
                            if count_key not in alarm_count:
                                alarm_count[count_key] = 0
                            alarm_count[count_key] += 1
                            
                            if alarm_count[count_key] >= 3:
                                # 打印告警信息
                                alarm = {
                                    "id": rule['id'],
                                    "tenantId": rule['tenantId'],
                                    "strategyId": rule['strategyId'],
                                    "hostId": rule['hostId'],
                                    "metricType": metric_type,
                                    "operator": operator,
                                    "triggerValue": str(trigger_value),
                                    "triggerTriggerValue": str(metric_value)
                                }
                                print("ALARM TRIGGERED:", json.dumps(alarm))
                                alarm_count[count_key] = 0
                        else:
                            # 重置计数
                            alarm_count[count_key] = 0
        except Exception as e:
            print("Error processing Kafka message:", str(e))

# 启动线程
thread1 = threading.Thread(target=sync_alarm_rules)
thread2 = threading.Thread(target=monitor_performance)

thread1.start()
thread2.start()

thread1.join()
thread2.join()
