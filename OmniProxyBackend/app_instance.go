package main

const singleInstanceIDBase = "com.omniproxy.desktop"

func singleInstanceUniqueID() string {
	return singleInstanceIDBase + "." + appInstanceMode
}
