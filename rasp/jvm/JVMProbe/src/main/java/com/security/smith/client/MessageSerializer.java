package com.security.smith.client;

import java.lang.reflect.Type;
import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonSerializationContext;
import com.google.gson.JsonSerializer;

import com.security.smith.common.ProcessHelper;

import java.lang.management.ManagementFactory;
import java.time.Instant;


public class MessageSerializer implements  JsonSerializer<Message> {
    static private int pid;
    static private String jvmVersion = "";
    static private String probeVersion = "";

    public static void initInstance(String probeVer) {
        pid = ProcessHelper.getCurrentPID();
        jvmVersion = ManagementFactory.getRuntimeMXBean().getSpecVersion();
        probeVersion = probeVer;
        if(probeVersion == null) {
            probeVersion = "";
        }
    }

    public static void delInstance() {
        jvmVersion = "";
        probeVersion = "";
    }

    public static void initInstance() {
        pid = ProcessHelper.getCurrentPID();
        jvmVersion = ManagementFactory.getRuntimeMXBean().getSpecVersion();
        if(jvmVersion == null) {
            jvmVersion = "";
        }

        probeVersion = MessageSerializer.class.getPackage().getImplementationVersion();
        if(probeVersion == null) {
            probeVersion = "";
        }
    }

    @Override
    public JsonElement serialize(Message message, Type typeOfSrc, JsonSerializationContext context) {
        JsonObject obj = new JsonObject();
        obj.addProperty("message_type", message.getOperate());
        obj.add("data", context.serialize(message.getData()));
        obj.addProperty("pid", pid);
        obj.addProperty("runtime", "JVM");
        obj.addProperty("runtime_version", jvmVersion);
        if(probeVersion != null) {
            obj.addProperty("probe_version", probeVersion);
        }
        else {
            obj.addProperty("probe_version", "");
        }
        obj.addProperty("time", Instant.now().getEpochSecond());
        return obj;
    }
}
