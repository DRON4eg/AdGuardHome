import React, { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { shallowEqual, useDispatch, useSelector } from 'react-redux';

import Card from '../../ui/Card';
import Loading from '../../ui/Loading';
import Form from './Form';

import { getDnsConfig, setDnsConfig } from '../../../actions/dnsConfig';
import { RootState } from '../../../initialState';

const Ipset: React.FC = () => {
    const { t } = useTranslation();
    const dispatch = useDispatch();

    const { processingGetConfig, processingSetConfig, ipset, ipset_file, ipset_create } = useSelector(
        (state: RootState) => ({
            processingGetConfig: state.dnsConfig.processingGetConfig,
            processingSetConfig: state.dnsConfig.processingSetConfig,
            ipset: state.dnsConfig.ipset || [],
            ipset_file: state.dnsConfig.ipset_file || '',
            ipset_create: state.dnsConfig.ipset_create || null,
        }),
        shallowEqual,
    );

    useEffect(() => {
        dispatch(getDnsConfig());
    }, [dispatch]);

    const handleSubmit = (data: { ipset: string[]; ipset_file: string; ipset_create: any }) => {
        dispatch(setDnsConfig(data));
    };

    if (processingGetConfig) {
        return <Loading />;
    }

    return (
        <Card title={t('ipset_title')} subtitle={t('ipset_info_desc')} bodyType="card-body box-body--settings">
            <Form
                initialRules={ipset}
                initialFilePath={ipset_file}
                initialIpsetCreate={ipset_create}
                onSubmit={handleSubmit}
                processing={processingSetConfig}
            />
        </Card>
    );
};

export default Ipset;
