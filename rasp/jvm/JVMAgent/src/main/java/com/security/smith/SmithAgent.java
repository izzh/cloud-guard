package com.security.smith;

import com.security.smith.log.SmithLogger;
import com.security.smith.common.JarUtil;
import com.security.smith.common.ParseParameter;
import com.security.smith.common.Reflection;

import java.util.jar.Attributes;
import java.util.jar.Manifest;

import java.lang.instrument.Instrumentation;
import java.util.concurrent.locks.ReentrantLock;

public class SmithAgent {
    private static ReentrantLock    xLoaderLock = new ReentrantLock();
    private static SmithLoader      xLoader = null;
    private static Class<?>         SmithProberClazz = null;
    private static Object           SmithProberObj = null; 
  
    private static boolean loadSmithProber(String proberPath, Instrumentation inst) {
        boolean bret = false;

        System.out.println("loadSmithProber Entry");

        try {
            xLoader = new SmithLoader(proberPath, null);
            SmithProberClazz = xLoader.loadClass("com.security.smith.SmithProbe");

            Class<?>[] emptyArgTypes = new Class[]{};
            SmithProberObj = Reflection.invokeStaticMethod(SmithProberClazz,"getInstance", emptyArgTypes);

            Class<?>[]  argType = new Class[]{Instrumentation.class};
            Reflection.invokeMethod(SmithProberObj,"setInst",argType,inst);
            Reflection.invokeMethod(SmithProberObj,"init",emptyArgTypes);
            Reflection.invokeMethod(SmithProberObj,"start",emptyArgTypes);

            bret = true;
        }
        catch(Exception e) {
            e.printStackTrace();
        }

        System.out.println("loadSmithProber Leave");

        return bret;
    }

    private static Boolean unLoadSmithProber() {
        boolean bret = false;

        System.out.println("unLoadSmithProber Entry");

        try {
            if(SmithProberObj != null) {
                System.out.println("Start unload prober");
                Class<?>[] emptyArgTypes = new Class[]{};
                Reflection.invokeMethod(SmithProberObj,"stop",emptyArgTypes);
                System.out.println("unload prober 0");
                Reflection.invokeMethod(SmithProberObj,"uninit",emptyArgTypes);
                System.out.println("unload prober 1");

                SmithProberObj = null;
                SmithProberClazz = null;
                xLoader = null;

                System.out.println("unload prober end");

                bret = true;
            }
        }
        catch(Exception e) {
            e.printStackTrace();
        }

        System.out.println("unLoadSmithProber Leave");

        return bret;
    }

    private static String getProberVersion(String jarFilePath) {
        try {
            java.util.jar.JarFile jarFile = new java.util.jar.JarFile(jarFilePath);
            Manifest manifest = jarFile.getManifest();
            Attributes mainAttributes = manifest.getMainAttributes();

            String ImplementationVersion = manifest.getMainAttributes().getValue("Implementation-Version");

            return ImplementationVersion;
        }
        catch(Exception e) {
            e.printStackTrace();
        }

        return null;
    }

    public static void premain(String agentArgs, Instrumentation inst) {
        agentmain(agentArgs, inst);
    }

    public static void agentmain(String agentArgs, Instrumentation inst) {
        String agent = System.getProperty("rasp.agent");

        System.out.println("agentArgs:"+agentArgs);

        StringBuilder cmd_sb = new StringBuilder();
        StringBuilder checksumStr_sb = new StringBuilder();
        StringBuilder proberPath_sb = new StringBuilder();
        String cmd = "";
        String checksumStr = "";
        String proberPath = "";

        if(ParseParameter.parseParameter(agentArgs,cmd_sb,checksumStr_sb,proberPath_sb)) {
            cmd = cmd_sb.toString();
            System.out.println("cmd:" + cmd);
            System.out.println("parse parseParameter success");

            if(cmd.equals("attach")) {
                checksumStr = checksumStr_sb.toString();
                proberPath = proberPath_sb.toString();
                
                System.out.println("checksumStr:" + checksumStr);
                System.out.println("proberPath:" + proberPath); 

                if(!JarUtil.checkJarFile(proberPath,checksumStr)) {
                    System.setProperty("smith.status", proberPath + " check fail!");
                    SmithLogger.logger.warning(proberPath + " check fail!");
                    return ;
                }

                String probeVersion = getProberVersion(proberPath);
                System.out.println("proberVersion:" + probeVersion);

                xLoaderLock.lock();
                try {
                    if(xLoader != null) {
                        if(unLoadSmithProber()) {
                            System.setProperty("smith.rasp", "");
                            System.setProperty("smith.status", "detach");
                        }
                        xLoader = null;
                        SmithProberObj = null;
                        SmithProberClazz = null;
                    }

                    System.setProperty("smith.rasp", "");

                    if(!loadSmithProber(proberPath,inst)) {
                        System.setProperty("smith.status",proberPath + " loading fail!");
                        SmithLogger.logger.warning(proberPath + " loading fail!");
                    }
                    else {
                        System.setProperty("smith.rasp", probeVersion+"-"+checksumStr);
                        System.setProperty("smith.status", "attach");
                    }
                }
                finally {
                    xLoaderLock.unlock();
                }
            }
            else if(cmd.equals("detach")) {
                xLoaderLock.lock();
                try {
                    if(xLoader != null) {
                        if(unLoadSmithProber()) {
                            System.setProperty("smith.rasp", "");
                            System.setProperty("smith.status", "detach");
                        }
                        else {
                            System.setProperty("smith.status", "prober unload fail");
                        }
                        xLoader = null;
                        SmithProberObj = null;
                        SmithProberClazz = null;
                    }
                    else {
                        SmithLogger.logger.warning("SmithProber No Loading!");
                    }
                }
                finally {
                    xLoaderLock.unlock();
                }
            } else {
                SmithLogger.logger.warning("Unknow Command:"+cmd);
                return ;
            }
        }
        else {
            System.out.println("parse parameter fail");
            return ;
        }

        

        if (agent != null) {
            SmithLogger.logger.info("agent running");
            return;
        }

        System.setProperty("rasp.agent", "smith");
    }
}
