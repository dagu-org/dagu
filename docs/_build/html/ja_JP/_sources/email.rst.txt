Sending Email Notifications
===========================

Email notifications can be sent when a DAG finished with an error or successfully. To do so, you can set the ``smtp`` field and related fields in the DAG specs. You can use any email delivery services (e.g. Sendgrid, Mailgun, etc).

.. code-block:: yaml

    # Email notification settings
    mailOn:
      failure: true
      success: true

    # SMTP server settings
    smtp:
      host: "smtp.foo.bar"
      port: "587"
      username: "<username>"
      password: "<password>"

    # Error mail configuration
    errorMail:
      from: "foo@bar.com"
      to: "foo@bar.com"
      prefix: "[Error]"
      attachLogs: true

    # Info mail configuration
    infoMail:
      from: "foo@bar.com"
      to: "foo@bar.com"
      prefix: "[Info]"
      attachLogs: true

If you want to use the same settings for all DAGs, set them to the :ref:`base configuration`.
