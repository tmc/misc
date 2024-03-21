'use client';

import { useState } from 'react';
import { debounce } from 'lodash';
import { MapCameraChangedEvent } from '@vis.gl/react-google-maps';
import { APIProvider, Map } from '@vis.gl/react-google-maps';

// import Container from "ui/components/Container";
// import Map from 'ui/components/Map';
// import Button from 'ui/components/Button';

const API_KEY = '';

const IndexLocationPage = () => {
  const [search, setSearch] = useState('');
  const [bounds, setBounds] = useState({
    nw: { lat: 0, lng: 0 },
    se: { lat: 0, lng: 0 },
  });
  const locations = [];

  const handleBoundsChanged = debounce((e: MapCameraChangedEvent) => {
    console.log('Bounds changed', e);
    // setBounds('TESTING');
    setBounds({});
  }, 200);

  console.log('RENDER');
  return (
    <APIProvider apiKey={API_KEY}>
      <Map
        style={{ width: '100vw', height: '100vh' }}
        defaultCenter={{ lat: 22.54992, lng: 0 }}
        defaultZoom={3}
        gestureHandling={'greedy'}
        disableDefaultUI={true}
        onBoundsChanged={handleBoundsChanged}
      />
    </APIProvider>
  );
};

export default IndexLocationPage;
