FROM gcr.io/distroless/static-debian11:nonroot
ENTRYPOINT ["/baton-confluence"]
COPY baton-confluence /