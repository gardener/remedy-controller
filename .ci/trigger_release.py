#!/usr/bin/env python3

import ci.util
import concourse.client


def trigger_release_job():
    concourse_cfg = ci.util.ctx().cfg_factory() \
      .cfg_set(ci.util.check_env('CONCOURSE_CURRENT_CFG')).concourse()
    concourse_api = concourse.client.from_cfg(
      concourse_cfg=concourse_cfg,
      team_name=ci.util.check_env('CONCOURSE_CURRENT_TEAM'),
    )

    # RELEASE_JOB_NAME must be passed via pipeline-definition
    job_name = ci.util.check_env('RELEASE_JOB_NAME')
    concourse_api.trigger_build(
      pipeline_name=ci.util.check_env('PIPELINE_NAME'),
      job_name=job_name,
    )
    ci.util.info(f'triggered release job {job_name}')


def main():
    trigger_release_job()


if __name__ == '__main__':
    main()
