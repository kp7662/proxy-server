# proxy-server
The goal of this project is to create a HTTP web proxy and cache in GoLang. The proxy will act as an intermediary for HTTP requests, controlling the flow and content of data between clients and servers. The project involves concepts of networking concepts, multithreading, and caching mechanisms.

**INSTRUCTIONS**

1 Clone the project repository on Github to your local computer.
   
2 **Running proxy server and client application in the same machine**: If you wish to run the proxy and client application on the same local machine, first, navigate to the project folder, open a terminal, and run the following command: go run proxy.go cache_without_lru.go blockedset.go. Then, open another terminal and run the following command: go run client.go URL. You can find example websites [here](https://www.androidauthority.com/sites-still-on-http-889265/). Then, you may inspect the output in the terminal. You should see the response body in the client terminal and server response message in the proxy terminal. 

3 **Running proxy server and client browser in different machines**: To provide a better user experience, we **strongly recommend** using a web browser (preferably Mozilla Firefox) as the client application and running the proxy in a separate computer (so that the IP addresses of the client and proxy are different). 

3.1. First, retrieve the IP address of the local machine that the proxy server will be running on by following the instructions [here](https://timesofindia.indiatimes.com/education/learning-with-toi/how-to-find-ip-address-on-windows-or-mac-a-step-by-step-guide/articleshow/103606854.cms). You should do this on the computer where you have the project repository on.

### For Windows:
- In the Command Prompt window, type “ipconfig” and press Enter.
- Look for the “IPv4 Address” or “IPv6 Address” under the network connection you are using. This is your computer’s IP address. 

### For Mac:
- In the Terminal window, type “ifconfig” and press Enter.
- Locate the section that corresponds to your active network connection (usually labeled “en0” or “en1”).
- Look for the “inet” line; the number next to it is your computer’s IP address.

3.2. Then, you need to configure the network settings in Mozilla Firefox. Go to Settings → Network Settings → Under “Configure Proxy Access to the Internet”, select “Manual proxy configuration”, use the IP address of the local machine that the proxy server will be running on as the “HTTP Proxy” box and ‘9999” as the port number. You may also check the box for “Also use this proxy for HTTPS.” It should look like this:
<img width="267" alt="Screenshot 2023-12-14 171116" src="https://github.com/kp7662/proxy-server/assets/124271891/291fc470-8f5b-4468-ba5f-3b5264dcdd10">

3.3. Once you’re done with the set-up, navigate to the main() function in proxy.go and update the IP address to be the one that the proxy server will be running on. Then, run the following command: go run proxy.go cache_without_lru.go blockedset.go.

3.4. Launch Mozilla Firefox on a different computer, check the network setting is configured to route HTTP requests to the proxy server by following the steps in 3.2. Now, you may visit any HTTP sites on the browser and observe the visual layout of the HTTP sites. The HTTP sites routed through the proxy server should look the same as the ones without a proxy server. This [website] (https://www.androidauthority.com/sites-still-on-http-889265/) has a compiled list of HTTP sites that you may try to access with our proxy server.

3.5. To inspect the network traffic, you may click “Inspect” and inspect each request entry manually and the headers as well. 

3.6. To load sites from **cache**, clear the cache on Mozilla Firefox **every time** you load a HTTP site. You may do so by going to Settings → Search “Cache” → Under “Cookies and Site Data,” click “Clear Data” → Click “Clear”. If you do not perform this step, the browser will load the sites automatically from its own cache rather than our cache files. You could check in the terminal whether the data was served from the cache or the destination server,  if the data is stale or not and the time difference between the cache and destination server. The headers are also printed in the terminal.

4 **Running Cache with LRU**: If you want to test cachelru.go, you can switch it with cache_without_lru.go. Also, for cachelru.go if you restart the proxy, you should delete the cached folder as well, the reason is explained in the write-up
   
5 **Accessing Blocked Sites**: You may try to access blocked websites specified in the blocked-domains.txt. The correct output should show “forbidden content.” Test this feature with this blocked HTTP site with our proxy server like: [gov.bg] (https://gov.bg/), or choose others that match the ones specified in blocked-domains.txt 

If you run into any problems, please email Kok Wei Pua (kp7662@princeton.edu) or Aylin Hadzhieva (ah4068@princeton.edu) to explain the situations, and we will help you troubleshoot the errors.


