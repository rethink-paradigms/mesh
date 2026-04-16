FROM nginx:alpine
# Check for secrets injection (in real app, app would read /secrets/env)
# Here we just serve a static page
RUN echo "<h1>Hello World from Nomad!</h1>" > /usr/share/nginx/html/index.html
