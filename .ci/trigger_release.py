#!/usr/bin/env python3

import ci.util
import ccc.concourse


def trigger_release_job():
    concourse_client = ccc.concourse.client_from_env()

    # RELEASE_JOB_NAME must be passed via pipeline-definition
    job_name = ci.util.check_env('RELEASE_JOB_NAME')

    concourse_client.trigger_build(
      pipeline_name=ci.util.check_env('PIPELINE_NAME'),
      job_name=job_name,
    )
    ci.util.info(f'triggered release job {job_name}')


def main():
    trigger_release_job()


if __name__ == '__main__':
    main()
